package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/kevinburke/ssh_config"
	"golang.org/x/crypto/ssh"
	kh "golang.org/x/crypto/ssh/knownhosts"
)

var (
	LINES, _   = strconv.Atoi(os.Getenv("LINES"))
	COLUMNS, _ = strconv.Atoi(os.Getenv("COLUMNS"))

	HOME, _ = os.UserHomeDir()

	KNOWNHOSTS = path.Join(HOME, ".ssh", "known_hosts")
	SSHCONFIG  = path.Join(HOME, ".ssh", "config")

	HostKeyCallBack, _ = kh.New(KNOWNHOSTS)
)

type SSHItem struct {
	Host         string
	Port         string
	User         string
	IdentityFile string
}

func getSSHConfig(host string) SSHItem {
	bytes, err := os.ReadFile(SSHCONFIG)
	if err != nil {
		log.Fatalf("unable to read ssh config: %v", err)
	}
	cfg, _ := ssh_config.DecodeBytes(bytes)

	var sshItem SSHItem

	sshItem.Host, _ = cfg.Get(host, "Hostname")
	sshItem.Port, _ = cfg.Get(host, "Port")
	sshItem.User, _ = cfg.Get(host, "User")
	sshItem.IdentityFile, _ = cfg.Get(host, "IdentityFile")

	return sshItem
}

func replacePathToHome(str string) string {
	if strings.HasPrefix(str, "~") {
		return strings.Replace(str, "~", HOME, 1)
	}
	return str
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("please input host")
	}
	host := os.Args[1]
	sshItem := getSSHConfig(host)
	if sshItem.Host == "" {
		log.Fatalf("host %s not found", host)
	}

	key, err := os.ReadFile(replacePathToHome(sshItem.IdentityFile))
	if err != nil {
		log.Fatalf("unable to read private key: %v", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		log.Fatalf("unable to parse private key: %v", err)
	}

	config := &ssh.ClientConfig{
		User: sshItem.User,
		Auth: []ssh.AuthMethod{
			// Use the PublicKeys method for remote authentication.
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: HostKeyCallBack,
	}

	// Connect to the remote server and perform the SSH handshake.
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%s", sshItem.Host, sshItem.Port), config)
	if err != nil {
		log.Fatalf("unable to connect: %v", err)
	}
	defer client.Close()

	ss, err := client.NewSession()
	if err != nil {
		log.Fatal("unable to create SSH session: ", err)
	}
	defer ss.Close()

	ss.Stdout = os.Stdout
	ss.Stderr = os.Stderr
	ss.Stdin = os.Stdin

	// Set up terminal modes
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,     // enable echoing
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}

	if err := ss.RequestPty("xterm", LINES, COLUMNS, modes); err != nil {
		log.Fatal("request for pseudo terminal failed: ", err)
	}

	if err := ss.Shell(); err != nil {
		log.Fatal("failed to start shell: ", err)
	}

	if err := ss.Wait(); err != nil {
		log.Fatal("failed to wait: ", err)
	}
}
