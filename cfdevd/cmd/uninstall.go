// +build darwin

package cmd

import (
	"net"

	"code.cloudfoundry.org/cfdev/launchd"
)

//go:generate mockgen -package mocks -destination mocks/launchd.go code.cloudfoundry.org/cfdev/cfdevd/cmd Launchd
type Launchd interface {
	RemoveDaemon(spec launchd.DaemonSpec) error
}

type UninstallCommand struct {
	Launchd Launchd
}

func (u *UninstallCommand) Execute(conn *net.UnixConn) error {
	spec := launchd.DaemonSpec{
		Label: "org.cloudfoundry.cfdevd",
	}
	err := u.Launchd.RemoveDaemon(spec)
	if err == nil {
		conn.Write([]byte{0})
	} else {
		conn.Write([]byte{1})
	}
	return err
}
