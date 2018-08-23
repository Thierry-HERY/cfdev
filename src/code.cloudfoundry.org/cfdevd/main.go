package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"io"

	"code.cloudfoundry.org/cfdevd/cmd"
	"code.cloudfoundry.org/cfdev/daemon"
	"flag"
	"time"
)

const SockName = "ListenSocket"

func handleRequest(conn *net.UnixConn) {
	if err := doHandshake(conn); err != nil {
		fmt.Println("Handshake Error: ", err)
		return
	}

	command, err := cmd.UnmarshalCommand(conn)
	if err != nil {
		fmt.Println("Command:", err)
		return
	}
	command.Execute(conn)
}

func registerSignalHandler() {
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
	go func(c chan os.Signal) {
		sig := <-c
		log.Printf("Caught signal %s: shutting down.", sig)
		os.Exit(0)
	}(sigc)
}

func install(programSrc string, args []string) {
	lctl := daemon.New("")
	program := "/Library/PrivilegedHelperTools/org.cloudfoundry.cfdevd"
	programArgs := []string{program}
	programArgs = append(programArgs, args...)
	cfdevdSpec := daemon.DaemonSpec{
		Label:            "org.cloudfoundry.cfdevd",
		Program:          program,
		ProgramArguments: programArgs,
		RunAtLoad:        false,
		Sockets: map[string]string{
			SockName: "/var/tmp/cfdevd.socket",
		},
		StdoutPath: "/var/tmp/cfdevd.stdout.log",
		StderrPath: "/var/tmp/cfdevd.stderr.log",
	}
	if err := copyExecutable(programSrc, program); err != nil {
		fmt.Println("Failed to copy cfdevd: ", err)
	}
	if err := lctl.AddDaemon(cfdevdSpec); err != nil {
		fmt.Println("Failed to install cfdevd: ", err)
	}
}

func copyExecutable(src string, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	target, err := os.Create(dest)
	if err != nil {
		return err
	}

	if err = os.Chmod(dest, 0744); err != nil {
		return err
	}

	binData, err := os.Open(src)
	if err != nil {
		return err
	}

	_, err = io.Copy(target, binData)
	return err
}

func uninstall(prog string) {
	lctl := daemon.New("")
	program := "/Library/PrivilegedHelperTools/org.cloudfoundry.cfdevd"
	if err := lctl.RemoveDaemon("org.cloudfoundry.cfdevd"); err != nil {
		fmt.Println("Failed to uninstall cfdevd: ", err)
	}
	if err := os.Remove(program); err != nil {
		fmt.Println("Failed to delete installed cfdevd:", err)
	}
}

func timesync(socket string) {
	for {
		fmt.Printf("dialing socket %s \n", socket)
		net.Dial("unix", socket)
		time.Sleep(10 * time.Second)
	}
}

func run() {
	var timesyncSocket = flag.String("timesyncSock", "", "unix socket for timesync")
	flag.Parse()
	if *timesyncSocket != "" {
		go timesync(*timesyncSocket)
	}
	registerSignalHandler()
	listeners, err := daemon.Listeners(SockName)
	if err != nil || len(listeners) != 1 {
		log.Fatal("Failed to obtain socket from launchd")
	}
	listener, ok := listeners[0].(*net.UnixListener)
	if !ok {
		log.Fatal("Failed to cast listener to unix listener")

	}
	for {
		conn, err := listener.AcceptUnix()
		if err != nil {
			continue
		}
		defer conn.Close()
		go handleRequest(conn)
	}
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "install":
			fmt.Printf("Installing args=%+v", os.Args)
			install(os.Args[0], os.Args[1:])
		case "uninstall":
			uninstall(os.Args[0])
		default:
			log.Fatal("unrecognized command ", os.Args[1])
		}
	} else {
		fmt.Printf("Running args=%+v", os.Args)
		run()
	}
}
