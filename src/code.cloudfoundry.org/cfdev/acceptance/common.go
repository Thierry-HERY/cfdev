package acceptance

import (
	"crypto/tls"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

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
		"cf-oss-deps.iso",
		"vpnkit",
		"hyperkit",
		"linuxkit",
		"UEFI.fd",
		"qcow-tool",
	}

	err := os.MkdirAll(cacheDir, 0777)
	Expect(err).ToNot(HaveOccurred())

	for _, asset := range assets {
		target := filepath.Join(cacheDir, asset)
		for _, origin := range []string{filepath.Join(gopaths[0], "output", asset), filepath.Join(gopaths[0], "linuxkit", asset), filepath.Join(os.Getenv("HOME"), ".cfdev", "cache", asset)} {
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

func PidFromFile(pidFile string) int {
	pidBytes, _ := ioutil.ReadFile(pidFile)
	pid, _ := strconv.ParseInt(string(pidBytes), 10, 64)
	return int(pid)
}

func HasSudoPrivilege() bool {
	cmd := exec.Command("sh", "-c", "sudo -n true")
	err := cmd.Run()

	if err == nil {
		return true
	}

	switch t := err.(type) {
	case *exec.ExitError:
		return t.Success()
	default:
		panic(err)
	}
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
