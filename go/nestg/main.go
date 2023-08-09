package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"
)

type Identifier struct {
	Name      string
	Docker    DockerFile
	BuildEnvs []string
}

type DockerFile struct {
	FromBase   string
	ExposePort string
	ExecCmd    string
	Runs       []string
}

func (d *DockerFile) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("FROM %s\n", d.FromBase))
	for _, v := range d.Runs {
		sb.WriteString(fmt.Sprintf("%s\n", v))
	}

	if d.ExposePort != "" {
		sb.WriteString(fmt.Sprintf("EXPOSE %s\n", d.ExposePort))
	}

	sb.WriteString(fmt.Sprintf(d.ExecCmd, "%s"))
	return sb.String()
}

var (
	exposePort string
	goLdflag   string
	dImgName   string

	AlapineI = Identifier{
		Name: "alpine",
		Docker: DockerFile{
			FromBase:   "alpine:latest",
			Runs:       vec("RUN mkdir /app", "WORKDIR /app", "COPY . ."),
			ExecCmd:    "CMD [\"/app/%s\"]",
			ExposePort: exposePort,
		},
		BuildEnvs: vec("CGO_ENABLED=0", "GOOS=linux"),
	}
)

func getBinaryName() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	file, err := os.OpenFile(dir+"/go.mod", os.O_RDONLY, 0644)
	if err != nil {
		if os.IsNotExist(err) {
			return path.Base(dir), nil
		}
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		modPath := strings.TrimSpace(scanner.Text()[7:])
		return path.Base(modPath), nil
	}

	return path.Base(dir), nil
}

func red(s string) string {
	return "\033[1;31m" + s + "\033[0m"
}

func blue(s string) string {
	return "\033[1;34m" + s + "\033[0m"
}

func orange(s string) string {
	return "\033[1;33m" + s + "\033[0m"
}

func init() {
	flag.StringVar(&exposePort, "p", "", "port")
	flag.StringVar(&dImgName, "i", "", "image name")
	flag.StringVar(&goLdflag, "b", "", "go build flags")
}

func getIdentifier() Identifier {
	return AlapineI
}

func main() {
	flag.Parse()
	apapine := getIdentifier()

	binName, err := getBinaryName()
	if err != nil {
		fmt.Printf("Scan binary file failed: %s\n", red(err.Error()))
	}

	if dImgName == "" {
		dImgName = "nestg" + "/" + binName + ":" + time.Now().Format("20060102150405")[8:]
	}

	fmt.Printf("Identifier: %s, Binary: %s, Image: %s\n", blue(apapine.Name), blue(binName), blue(dImgName))

	tmpf, err := os.CreateTemp("", "nestg-*.dockerfile")
	if err != nil {
		fmt.Printf("Temp file create error: %s\n", red(err.Error()))
		return
	}

	data := fmt.Sprintf(apapine.Docker.String(), binName)

	tmpf.WriteString(data)
	defer os.Remove(tmpf.Name())

	fmt.Printf("Dockerfile:\n%s\n", orange(data))

	cmd := exec.Command("go", "build", "-o", binName, ".")
	if goLdflag != "" {
		cmd = exec.Command("go", "build", "-ldflags", goLdflag, "-o", binName, ".")
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, apapine.BuildEnvs...)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Build error: %s\n", red(err.Error()))
		return
	}

	cmd = exec.Command("docker", "build", "-t", dImgName, "-f", tmpf.Name(), ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Build image error: %s\n", red(err.Error()))
		return
	}

	if exposePort != "" {
		fmt.Printf("Run: %s\n", orange("docker run -it --rm -p "+"port:"+exposePort+" "+dImgName))
		return
	}
	fmt.Printf("Run: %s\n", orange("docker run -it --rm "+dImgName))
}

func vec(s ...string) []string {
	return s
}
