package privileged_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"

	"os"

	"github.com/onsi/gomega/gexec"
)

var pluginPath string

func TestPrivileged(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "cf dev - acceptance - privileged suite")
}

var _ = BeforeSuite(func() {
	pluginPath = os.Getenv("CFDEV_PLUGIN_PATH")
	if pluginPath == "" {
		var err error
		pluginPath, err = gexec.Build("code.cloudfoundry.org/cfdev")
		Expect(err).ShouldNot(HaveOccurred())
	}
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})
