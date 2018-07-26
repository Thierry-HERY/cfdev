package launchd_test

import (
	"code.cloudfoundry.org/cfdev/launchd"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	lnchd    launchd.Launchd
	label    string
	tmpDir   string
	assetDir string
)

var _ = Describe("launchd windows", func() {
	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "testasset")
		Expect(err).To(BeNil())

		assetDir := filepath.Join(tmpDir, "cache")

		err = os.MkdirAll(filepath.Join(tmpDir, "cache"), 0666)
		Expect(err).To(BeNil())

		assetPath := filepath.Join(assetDir, "winsw.exe")

		err = downloadTestAsset(assetPath, "https://github.com/kohsuke/winsw/releases/download/winsw-v2.1.2/WinSW.NET4.exe")
		Expect(err).To(BeNil())
		Expect(assetPath).To(BeAnExistingFile())
	})

	AfterEach(func() {
		err := os.RemoveAll(tmpDir)
		Expect(err).To(BeNil())
	})

	Describe("launchd windows", func() {
		BeforeEach(func() {
			lnchd = launchd.Launchd{}
			label = "some-daemon"
		})

		AfterEach(func() {
			spec := launchd.DaemonSpec{
				Label:     label,
				CfDevHome: tmpDir,
			}
			lnchd.Stop(spec)
			lnchd.RemoveDaemon(spec)
		})

		Describe("AddDaemon", func() {
			It("should load the daemon", func() {
				spec := launchd.DaemonSpec{
					Label:            label,
					CfDevHome:        tmpDir,
					Program:          "powershell.exe",
					ProgramArguments: []string{"echo 'hello'"},
				}

				Expect(lnchd.AddDaemon(spec)).To(Succeed())
				output := getPowerShellOutput(fmt.Sprintf(`Get-Service | Where-Object { $_.Name -eq "%s" }`, label))

				Expect(output).NotTo(BeEmpty())
			})
		})

		Describe("RemoveDaemon", func() {
			It("should remove the daemon", func() {
				spec := launchd.DaemonSpec{
					Label:     label,
					CfDevHome: tmpDir,
					Program:   "powershell.exe",
				}

				Expect(lnchd.AddDaemon(spec)).To(Succeed())
				output := getPowerShellOutput("get-service")
				Expect(output).To(ContainSubstring(label))

				Expect(lnchd.RemoveDaemon(spec)).To(Succeed())
				output = getPowerShellOutput(fmt.Sprintf(`Get-Service | Where-Object { $_.Name -eq "%s" }`, label))
				Expect(output).To(BeEmpty())
			})
		})

		Describe("Lifecycle", func() {
			var testFilePath string

			BeforeEach(func() {
				testFilePath = filepath.Join(tmpDir, "test-file.txt")
			})

			AfterEach(func() {
				os.RemoveAll(testFilePath)
			})

			It("should start, stop the daemon", func() {
				By("adding the service")
				spec := launchd.DaemonSpec{
					Label:     label,
					CfDevHome: tmpDir,
					Program:   "powershell.exe",
					ProgramArguments: []string{
						fmt.Sprintf("'some-content' >> %s;", testFilePath),
						"Start-Sleep -Seconds 20",
					},
				}
				Expect(lnchd.AddDaemon(spec)).To(Succeed())

				By("starting the service")
				Expect(lnchd.Start(spec)).To(Succeed())
				Eventually(func() bool {
					isRunning, _ := lnchd.IsRunning(spec)
					return isRunning
				}, 20, 1).Should(BeTrue())

				Eventually(testFilePath).Should(BeAnExistingFile())

				By("stopping the service")
				Expect(lnchd.Stop(spec)).To(Succeed())
				Expect(lnchd.IsRunning(spec)).To(BeFalse())
			})
		})
	})
})

func downloadTestAsset(filepath string, url string) error {
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func getPowerShellOutput(command string) string {
	cmd := exec.Command("powershell.exe", "-Command", command)
	output, err := cmd.Output()
	Expect(err).NotTo(HaveOccurred())
	return strings.TrimSpace(string(output))
}
