// +build darwin

package cmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"code.cloudfoundry.org/cfdev/cfanalytics"
	"code.cloudfoundry.org/cfdev/config"
	"code.cloudfoundry.org/cfdev/env"
	gdn "code.cloudfoundry.org/cfdev/garden"
	"code.cloudfoundry.org/cfdev/network"
	"code.cloudfoundry.org/cfdev/process"
	"github.com/spf13/cobra"
)

type UI interface {
	Say(message string, args ...interface{})
	Writer() io.Writer
}

type start struct {
	Exit        chan struct{}
	UI          UI
	Config      config.Config
	Registries  string
	DepsIsoPath string
	Cpus        int
	Mem         int
}

func NewStart(Exit chan struct{}, UI UI, Config config.Config) *cobra.Command {
	s := start{Exit: Exit, UI: UI, Config: Config}
	cmd := &cobra.Command{
		Use: "start",
		RunE: func(cmd *cobra.Command, args []string) error {
			return s.RunE()
		},
	}
	pf := cmd.PersistentFlags()
	pf.StringVarP(&s.DepsIsoPath, "file", "f", "", "path to .dev file containing bosh & cf bits")
	pf.StringVarP(&s.Registries, "registries", "r", "", "docker registries that skip ssl validation - ie. host:port,host2:port2")
	pf.IntVarP(&s.Cpus, "cpus", "c", 4, "cpus to allocate to vm")
	pf.IntVarP(&s.Mem, "memory", "m", 4096, "memory to allocate to vm in MB")

	return cmd
}

func (s *start) RunE() error {
	go func() {
		<-s.Exit
		process.SignalAndCleanup(s.Config.LinuxkitPidFile, s.Config.CFDevHome, syscall.SIGTERM)
		process.SignalAndCleanup(s.Config.VpnkitPidFile, s.Config.CFDevHome, syscall.SIGTERM)
		process.SignalAndCleanup(s.Config.HyperkitPidFile, s.Config.CFDevHome, syscall.SIGKILL)
		os.Exit(128)
	}()

	s.Config.Analytics.Event(cfanalytics.START_BEGIN, map[string]interface{}{"type": "cf"})

	if err := env.Setup(s.Config); err != nil {
		return err
	}

	if isLinuxKitRunning(s.Config.LinuxkitPidFile) {
		s.UI.Say("CF Dev is already running...")
		s.Config.Analytics.Event(cfanalytics.START_END, map[string]interface{}{"type": "cf", "alreadyrunning": true})
		return nil
	}

	registries, err := s.parseDockerRegistriesFlag(s.Registries)
	if err != nil {
		return fmt.Errorf("Unable to parse docker registries %v\n", err)
	}

	vpnKit := process.VpnKit{
		Config: s.Config,
	}
	vCmd := vpnKit.Command()

	linuxkit := process.LinuxKit{
		Config:      s.Config,
		DepsIsoPath: s.DepsIsoPath,
	}

	UpdateCatalog(map[string]string{"file": s.DepsIsoPath}, s.Config.Dependencies.Items)
	if err = download(s.Config.Dependencies, s.Config.CacheDir, s.UI.Writer()); err != nil {
		return err
	}

	lCmd, err := linuxkit.Command(s.Cpus, s.Mem)
	if err != nil {
		return fmt.Errorf("Unable to find .dev file %v\n", err)
	}

	if err = cleanupStateDir(s.Config.StateDir); err != nil {
		return err
	}

	if err = s.setupNetworking(); err != nil {
		return err
	}

	if !process.IsCFDevDInstalled(s.Config.CFDevDSocketPath, s.Config.CFDevDInstallationPath, s.Config.Dependencies.Lookup("cfdevd").MD5) {
		if err := process.InstallCFDevD(s.Config.CacheDir); err != nil {
			return err
		}
	}

	if err = vpnKit.SetupVPNKit(); err != nil {
		return err
	}

	s.UI.Say("Starting VPNKit ...")
	if err := vCmd.Start(); err != nil {
		return fmt.Errorf("Failed to start VPNKit process: %v\n", err)
	}

	err = ioutil.WriteFile(s.Config.VpnkitPidFile, []byte(strconv.Itoa(vCmd.Process.Pid)), 0777)
	if err != nil {
		return fmt.Errorf("Failed to write vpnKit pid file: %v\n", err)
	}

	s.UI.Say("Starting the VM...")
	lCmd.Stdout, err = os.Create(filepath.Join(s.Config.CFDevHome, "linuxkit.log"))
	if err != nil {
		return fmt.Errorf("Failed to open logfile: %v\n", err)
	}
	lCmd.Stderr = lCmd.Stdout
	if err := lCmd.Start(); err != nil {
		return fmt.Errorf("Failed to start VM process: %v\n", err)
	}

	err = ioutil.WriteFile(s.Config.LinuxkitPidFile, []byte(strconv.Itoa(lCmd.Process.Pid)), 0777)
	if err != nil {
		return fmt.Errorf("Failed to write VM pid file: %v\n", err)
	}

	garden, err := gdn.NewClient(s.Config)
	if err != nil {
		return err
	}
	gdn.WaitForGarden(garden, 3*time.Minute)

	s.UI.Say("Deploying the BOSH Director...")
	if err := gdn.DeployBosh(s.Config, garden); err != nil {
		return fmt.Errorf("Failed to deploy the BOSH Director: %v\n", err)
	}

	s.UI.Say("Deploying CF...")
	if err := gdn.DeployCloudFoundry(garden, registries); err != nil {
		return fmt.Errorf("Failed to deploy the Cloud Foundry: %v\n", err)
	}
	s.UI.Say(cfdevStartedMessage)

	s.Config.Analytics.Event(cfanalytics.START_END, map[string]interface{}{"type": "cf"})

	return nil
}

func isLinuxKitRunning(pidFile string) bool {
	fileBytes, err := ioutil.ReadFile(pidFile)
	if err != nil {
		return false
	}

	pid, err := strconv.ParseInt(string(fileBytes), 10, 64)
	if err != nil {
		return false
	}

	if process, err := os.FindProcess(int(pid)); err == nil {
		err = process.Signal(syscall.Signal(0))
		return err == nil
	}

	return false
}

func cleanupStateDir(stateDir string) error {
	if err := os.RemoveAll(stateDir); err != nil {
		return fmt.Errorf("Unable to clean up .cfdev state directory: %v\n", err)
	}

	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("Unable to create .cfdev state directory: %v\n", err)
	}

	return nil
}

func (s *start) setupNetworking() error {
	err := network.AddLoopbackAliases(s.Config.BoshDirectorIP, s.Config.CFRouterIP)

	if err != nil {
		return fmt.Errorf("Unable to alias BOSH Director/CF Router IP: %v\n", err)
	}

	return nil
}

func (s *start) parseDockerRegistriesFlag(flag string) ([]string, error) {
	if flag == "" {
		return nil, nil
	}

	values := strings.Split(flag, ",")

	registries := make([]string, 0, len(values))

	for _, value := range values {
		// Including the // will cause url.Parse to validate 'value' as a host:port
		u, err := url.Parse("//" + value)

		if err != nil {
			// Grab the more succinct error message
			if urlErr, ok := err.(*url.Error); ok {
				err = urlErr.Err
			}
			return nil, fmt.Errorf("'%v' - %v", value, err)
		}
		registries = append(registries, u.Host)
	}
	return registries, nil
}
