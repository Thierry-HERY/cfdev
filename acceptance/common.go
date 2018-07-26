package acceptance

import (
	"archive/tar"
	"crypto/tls"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"code.cloudfoundry.org/cfdev/errors"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/client"
	"fmt"
	"runtime"

	. "github.com/onsi/gomega"
)

const (
	GardenIP       = "localhost"
	BoshDirectorIP = "10.245.0.2"
	CFRouterIP     = "10.144.0.34"
)

func SetupDependencies(cacheDir string) {
	gopaths := strings.Split(os.Getenv("GOPATH"), ":")

	assets := []string{
		"cfdev-efi.iso",
		"cf-deps.iso",
		"vpnkit",
		"hyperkit",
		"linuxkit",
		"UEFI.fd",
		"qcow-tool",
	}

	if runtime.GOOS == "windows" {
		assets = []string{
			"cfdev-efi.iso",
			"cf-deps.iso",
			"vpnkit.exe",
			"winsw.exe",
		}
	}

	err := os.MkdirAll(cacheDir, 0777)
	Expect(err).ToNot(HaveOccurred())

	for _, asset := range assets {
		target := filepath.Join(cacheDir, asset)

		goPath := gopaths[0]
		if runtime.GOOS == "windows" {
			goPath = os.Getenv("GOPATH")
		}

		for _, origin := range []string{filepath.Join(goPath, "output", asset), filepath.Join(goPath, "linuxkit", asset), filepath.Join(GetCfdevHome(), "cache", asset)} {
			if exists, _ := FileExists(origin); exists {
				Expect(os.Symlink(origin, target)).To(Succeed())
				break
			}
		}
		Expect(target).To(BeAnExistingFile())
	}
}

func EventuallyShouldListenAt(url string, timeoutSec int) {
	Eventually(func() error {
		return HttpServerIsListeningAt(url)
	}, timeoutSec, 1).ShouldNot(HaveOccurred())
}

func HttpServerIsListeningAt(url string) error {
	client := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	resp, err := client.Get(url)

	if resp != nil {
		resp.Body.Close()
	}

	return err
}

func EventuallyProcessStops(pid int, timeoutSec int) {
	EventuallyWithOffset(1, func() (bool, error) {
		return ProcessIsRunning(pid)
	}, timeoutSec).Should(BeFalse())
}

func ProcessIsRunning(pid int) (bool, error) {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, err
	}

	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return false, nil
	}

	return true, nil
}

//func isProcessRunning(label string) func() (bool, error) {
//	if runtime.GOOS == "windows" {
//		return func() (bool, error) {
//
//		}
//	} else {
//		return IsLaunchdRunning(label)
//	}
//}

func IsLaunchdRunning(label string) func() (bool, error) {
	return func() (bool, error) {
		if runtime.GOOS == "darwin" {
			txt, err := exec.Command("launchctl", "list", label).CombinedOutput()
			if err != nil {
				if strings.Contains(string(txt), "Could not find service") {
					return false, nil
				}
				return false, err
			}
			re := regexp.MustCompile(`^\s*"PID"\s*=`)
			for _, line := range strings.Split(string(txt), "\n") {
				if re.MatchString(line) {
					return true, nil
				}
			}
			return false, nil
		} else {
			cmd := exec.Command("powershell.exe", "-Command", fmt.Sprintf("Get-Service | Where-Object {$_.Name -eq \"%s\"}", label))
			output, err := cmd.Output()
			if err != nil {
				return false, err
			}

			if strings.Contains(string(output), label) {
				return true, nil
			}

			return false, nil
		}
	}
}

func PidFromFile(pidFile string) int {
	pidBytes, _ := ioutil.ReadFile(pidFile)
	pid, _ := strconv.ParseInt(string(pidBytes), 10, 64)
	return int(pid)
}

func FileExists(file string) (bool, error) {
	_, err := os.Stat(file)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func GetFile(client client.Client, handle, path string) (string, error) {
	c, err := client.Lookup(handle)
	if err != nil {
		return "", err
	}
	fh, err := c.StreamOut(garden.StreamOutSpec{
		Path: path,
	})
	if err != nil {
		return "", err
	}
	tr := tar.NewReader(fh)

	_, err = tr.Next()
	if err == io.EOF {
		return "", errors.SafeWrap(nil, "file not found")
	}
	if err != nil {
		return "", err
	}
	b, err := ioutil.ReadAll(tr)
	return string(b), err
}

func GetCfdevHome() string {
	cfdevHome := os.Getenv("CFDEV_HOME")
	if cfdevHome != "" {
		return cfdevHome
	}

	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("HOMEDRIVE"), os.Getenv("HOMEPATH"), ".cfdev")
	} else {
		return filepath.Join(os.Getenv("HOME"), ".cfdev")
	}
}

func GetCfPluginPath() string {
	if runtime.GOOS == "windows" {
		return "cf"
	} else {
		return "/usr/local/bin/cf"
	}
}
