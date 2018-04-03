package cmd_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"code.cloudfoundry.org/cfdevd/cmd"
	"net"
	"io/ioutil"
	"os"
	"path/filepath"
)

var _ = Describe("UnimplementedCommand", func() {
	Describe("Execute", func() {
		var (
			ln        *net.UnixListener
			socketDir string
			addr      *net.UnixAddr
		)

		BeforeEach(func() {
			var err error
			socketDir, err = ioutil.TempDir(os.Getenv("TMPDIR"), "socket")
			Expect(err).NotTo(HaveOccurred())
			addr = &net.UnixAddr{
				Name: filepath.Join(socketDir, "some.socket"),
			}
			ln, err = net.ListenUnix("unix", addr)
		})

		AfterEach(func() {
			Expect(os.RemoveAll(socketDir)).To(Succeed())
			Expect(ln.Close()).To(Succeed())
		})

		It("returns error code 33", func(done Done) {
			unimplemented := &cmd.UnimplementedCommand{}
			go func() {
				defer GinkgoRecover()
				conn, err := ln.Accept()
				Expect(err).NotTo(HaveOccurred())
				defer conn.Close()
				data := make([]byte, 1, 1)
				_, err = conn.Read(data)
				Expect(err).NotTo(HaveOccurred())
				Expect(data[0]).To(Equal(uint8(33)))
				close(done)
			}()
			conn, err := net.DialUnix("unix", nil, addr)
			Expect(err).NotTo(HaveOccurred())
			defer conn.Close()
			Expect(unimplemented.Execute(conn)).To(Succeed())
		})

		It("returns an error when it cannot write message", func() {
			unimplemented := &cmd.UnimplementedCommand{}
			conn, err := net.DialUnix("unix", nil, addr)
			Expect(err).NotTo(HaveOccurred())
			conn.Close()
			Expect(unimplemented.Execute(conn).Error()).To(ContainSubstring("failed to write error code to connection"))
		})
	})
})
