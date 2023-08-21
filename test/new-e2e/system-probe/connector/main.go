package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"time"

	sshtools "github.com/scylladb/go-sshtools"
	"golang.org/x/crypto/ssh"
)

type Args struct {
	host                    string
	user                    string
	port                    int
	serverKeepAliveInterval time.Duration
	serverKeepAliveMaxCount int
	sshFilePath             string
}

func readArgs() *Args {
	userPtr := flag.String("user", "", "SSH user")
	hostPtr := flag.String("host", "", "Host ip to connect to")
	portPtr := flag.Int("port", 22, "Port for ssh server")
	serverAlivePtr := flag.Int("server-alive-interval", 5, "Interval at which to send keep alive messages")
	serverAliveCountPtr := flag.Int("server-alive-count", 560, "Maximum keep alive messages to send before disconnecting upon no reply")
	sshFilePathPtr := flag.String("ssh-file", "", "Path to private ssh key")

	flag.Parse()

	return &Args{
		host:                    *hostPtr,
		user:                    *userPtr,
		port:                    *portPtr,
		serverKeepAliveInterval: time.Duration(*serverAlivePtr) * time.Second,
		serverKeepAliveMaxCount: *serverAliveCountPtr,
		sshFilePath:             *sshFilePathPtr,
	}
}

func main() {
	var cmd sshtools.Cmd

	args := readArgs()

	key, err := os.ReadFile(args.sshFilePath)
	if err != nil {
		log.Fatalf("unable to read private key: %v", err)
	}

	config := sshtools.Config{
		Port:                args.port,
		ServerAliveInterval: args.serverKeepAliveInterval,
		ServerAliveCountMax: args.serverKeepAliveMaxCount,
	}
	config, err = config.WithIdentityFileAuth(args.user, key)
	if err != nil {
		log.Fatal(err)
	}
	config.HostKeyCallback = ssh.InsecureIgnoreHostKey()

	communicator := sshtools.NewCommunicator(args.host, config, sshtools.ContextDialer(&net.Dialer{}), log.New(os.Stdout, "", log.LstdFlags))

	ctx := context.Background()
	if err := communicator.Connect(ctx); err != nil {
		log.Fatal(err)
	}

	cmd.Command = os.Getenv("VM_CMD")
	cmd.Stdout = os.Stdout
	if err := communicator.Start(ctx, &cmd); err != nil {
		log.Fatal(err)
	}

	if err := cmd.Wait(); err != nil {
		log.Fatal(err)
	}
}
