package process

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/cfdev/config"
	"code.cloudfoundry.org/cfdevd/launchd"
)

type UI interface {
	Say(message string, args ...interface{})
	Writer() io.Writer
}

type LinuxKit struct {
	Config      config.Config
	DepsIsoPath string
	Launchd     Launchd
}

type Launchd interface {
	AddDaemon(launchd.DaemonSpec) error
	Start(label string) error
}

const LinuxKitLabel = "org.cloudfoundry.cfdev.linuxkit"

func (l *LinuxKit) Start(cpus int, mem int) error {
	daemonSpec, err := l.daemonSpec(cpus, mem)
	if err != nil {
		return err
	}
	if err := l.Launchd.AddDaemon(daemonSpec); err != nil {
		return err
	}
	return l.Launchd.Start(LinuxKitLabel)
}

func (l *LinuxKit) daemonSpec(cpus, mem int) (launchd.DaemonSpec, error) {
	linuxkit := filepath.Join(l.Config.CacheDir, "linuxkit")
	hyperkit := filepath.Join(l.Config.CacheDir, "hyperkit")
	uefi := filepath.Join(l.Config.CacheDir, "UEFI.fd")
	qcowtool := filepath.Join(l.Config.CacheDir, "qcow-tool")
	vpnkitEthSock := filepath.Join(l.Config.VpnkitStateDir, "vpnkit_eth.sock")
	vpnkitPortSock := filepath.Join(l.Config.VpnkitStateDir, "vpnkit_port.sock")

	if l.DepsIsoPath == "" {
		l.DepsIsoPath = filepath.Join(l.Config.CacheDir, "cf-deps.iso")
	} else {
		if _, err := os.Stat(l.DepsIsoPath); os.IsNotExist(err) {
			return launchd.DaemonSpec{}, err
		}
	}

	dependencyImagePath := l.DepsIsoPath
	osImagePath := filepath.Join(l.Config.CacheDir, "cfdev-efi.iso")

	diskArgs := []string{
		"type=qcow",
		"size=80G",
		"trim=true",
		fmt.Sprintf("qcow-tool=%s", qcowtool),
		"qcow-onflush=os",
		"qcow-compactafter=262144",
		"qcow-keeperased=262144",
	}

	return launchd.DaemonSpec{
		Label:       LinuxKitLabel,
		Program:     linuxkit,
		SessionType: "Background",
		ProgramArguments: []string{
			linuxkit, "run", "hyperkit",
			"-console-file",
			"-cpus", fmt.Sprintf("%d", cpus),
			"-mem", fmt.Sprintf("%d", mem),
			"-hyperkit", hyperkit,
			"-networking", fmt.Sprintf("vpnkit,%v,%v", vpnkitEthSock, vpnkitPortSock),
			"-fw", uefi,
			"-disk", strings.Join(diskArgs, ","),
			"-disk", "file=" + dependencyImagePath,
			"-state", l.Config.StateDir,
			"--uefi",
			osImagePath,
		},
		RunAtLoad:  false,
		StdoutPath: path.Join(l.Config.CFDevHome, "linuxkit.stdout.log"),
		StderrPath: path.Join(l.Config.CFDevHome, "linuxkit.stderr.log"),
	}, nil
}
