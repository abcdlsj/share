package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/abcdlsj/cr"
	"golang.org/x/mod/modfile"
)

type Cmd struct {
	bin  string
	args []string
}

func (c *Cmd) Run() error {
	fmt.Printf("%s %s\n", cr.PLCyan(c.bin), cr.PLBlue(strings.Join(c.args, " ")))
	cmd := exec.Command(c.bin, c.args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}

func newCmd(bin string, args ...string) *Cmd {
	return &Cmd{
		bin:  bin,
		args: args,
	}
}

func main() {
	if len(os.Args) < 2 {
		return
	}

	if os.Args[1] == "-install" {
		installGov(os.Args[2])
		return
	}

	gv := "go"

	if strings.HasPrefix(os.Args[1], "-go") {
		gv = os.Args[1][1:]
		os.Args = os.Args[1:]
	} else {
		_, err := os.Stat("go.mod")
		if err != nil {
			if os.IsNotExist(err) {
				return
			}
			return
		}

		data, err := os.ReadFile("go.mod")
		if err != nil {
			return
		}

		f, err := modfile.Parse("go.mod", data, nil)
		if err != nil {
			panic(err)
		}

		gv = fmt.Sprintf("go%s", f.Go.Version)
	}

	if _, err := exec.LookPath(gv); err != nil {
		fmt.Printf("%s not found, start download...\n", cr.PLGreen(gv))
		installGov(gv)
	}

	newCmd(gv, os.Args[1:]...).Run()
}

func installGov(gv string) {
	newCmd("go", "install", fmt.Sprintf("golang.org/dl/%s@latest", gv)).Run()
	newCmd(gv, "download").Run()
}
