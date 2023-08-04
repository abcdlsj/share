package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"
)

type Identifier struct {
	Name      string
	DockerF   string
	BuildEnvs []string
}

var (
	AlapineIdentifer = Identifier{
		Name: "alpine",

		DockerF: `
FROM alpine:latest

RUN mkdir /app
WORKDIR /app
COPY . .

CMD ["/app/%s"]`,

		BuildEnvs: vec("CGO_ENABLED=0", "GOOS=linux"),
	}
)

func binName() (string, error) {
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
		return strings.TrimSpace(scanner.Text()[7:]), nil
	}

	return path.Base(dir), nil
}

func red(s string) string {
	return "\033[1;31m" + s + "\033[0m"
}

func blue(s string) string {
	return "\033[1;34m" + s + "\033[0m"
}

func main() {
	identifier := AlapineIdentifer
	fmt.Printf("Identifier: %s\n", blue(identifier.Name))

	binName, err := binName()
	if err != nil {
		fmt.Printf("Get binary name error: %s\n", red(err.Error()))
	}

	imageName := "nestg" + "/" + binName + ":" + time.Now().Format("20060102150405")[8:]

	tmpf, err := os.CreateTemp("", "nestg-*.dockerfile")
	if err != nil {
		fmt.Printf("Create temp file error: %s\n", red(err.Error()))
		return
	}

	data := fmt.Sprintf(identifier.DockerF, binName)

	tmpf.WriteString(data)
	defer os.Remove(tmpf.Name())

	fmt.Printf("Dockerfile:%s\n", blue(data))

	cmd := exec.Command("go", "build", "-o", binName, ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, identifier.BuildEnvs...)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Build error: %s\n", red(err.Error()))
		return
	}

	cmd = exec.Command("docker", "build", "-t", imageName, "-f", tmpf.Name(), ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Build image error: %s\n", red(err.Error()))
		return
	}

	fmt.Printf("Image: %s\n", blue(imageName))
	fmt.Printf("Usage: %s\n" + blue("docker run -it --rm "+imageName))
}

func vec(s ...string) []string {
	return s
}
