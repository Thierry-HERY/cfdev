// +build darwin

package cmd

import (
	"encoding/binary"
	"io"
	"net"
	"os"
	"code.cloudfoundry.org/cfdev/daemon"
)

type Command interface {
	Execute(*net.UnixConn) error
}

const UninstallType = uint8(1)
const RemoveIPAliasType = uint8(2)
const AddIPAliasType = uint8(3)
const BindType = uint8(6)

func UnmarshalCommand(conn io.Reader) (Command, error) {
	var instr uint8
	binary.Read(conn, binary.LittleEndian, &instr)

	switch instr {
	case BindType:

		return UnmarshalBindCommand(conn)
	case UninstallType:

		return &UninstallCommand{
			DaemonRunner: daemon.New(""),
		}, nil
	case RemoveIPAliasType:

		return &RemoveIPAliasCommand{}, nil
	case AddIPAliasType:

		return &AddIPAliasCommand{}, nil
	default:
		return &UnimplementedCommand{
			Instruction: instr,
			Logger: os.Stdout,
		}, nil
	}
}
