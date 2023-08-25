package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Vars  map[string]string `yaml:"vars"`
	Tasks map[string]Task   `yaml:"tasks"`
}

type Task struct {
	Pres []string          `yaml:"pres"`
	Cmds []string          `yaml:"cmds"`
	Vars map[string]string `yaml:"vars"`
}

var (
	taskName   string
	configFile string
	globalflag bool

	HomeDir, _   = os.UserHomeDir()
	globlCfgFile = filepath.Join(HomeDir, ".taky.yaml")

	taskConfig Config
)

func init() {
	flag.StringVar(&taskName, "task", "", "Name of the task to execute")
	flag.StringVar(&configFile, "file", "", "Path to the YAML config file")
	flag.BoolVar(&globalflag, "g", false, "Use the global config file")

	flag.Parse()
}

func main() {
	flag.Parse()

	if globalflag {
		configFile = globlCfgFile
	}

	if configFile == "" {
		if _, err := os.Stat(".taky.yaml"); err == nil {
			configFile = ".taky.yaml"
		} else {
			if _, err := os.Stat(globlCfgFile); err == nil {
				configFile = globlCfgFile
			}
		}
	}

	if taskName == "" {
		fmt.Println("Please specify a task name")
		return
	}

	configData, err := os.ReadFile(configFile)
	if err != nil {
		fmt.Printf("Failed to read config file, %v\n", err)
		return
	}

	err = yaml.Unmarshal(configData, &taskConfig)
	if err != nil {
		fmt.Printf("Failed to parse config file, %v\n", err)
		return
	}

	globalVars := map[string]string{}
	for key, value := range taskConfig.Vars {
		globalVars[key] = os.ExpandEnv(value)
	}

	task, ok := taskConfig.Tasks[taskName]
	if !ok {
		fmt.Printf("Task %s not found\n", taskName)
		return
	}

	if err := taskExec(task, taskName, globalVars); err != nil {
		fmt.Printf("Failed to execute task, %v\n", err)
		return
	}

}

func taskExec(task Task, taskName string, globalVars map[string]string) error {
	vars := map[string]string{}
	for key, value := range task.Vars {
		vars[key] = os.ExpandEnv(value)
	}

	for _, pre := range task.Pres {
		_, ok := taskConfig.Tasks[pre]
		if !ok {
			fmt.Printf("Task %s not found\n", pre)
			return errors.New("task not found")
		}

		if err := taskExec(taskConfig.Tasks[pre], pre, globalVars); err != nil {
			return err
		}
	}

	for idx, cmd := range task.Cmds {
		if err := taskExecCmd(cmd, taskName, globalVars, vars); err != nil {
			log.Fatalf("Failed to execute command: %v", err)
		}

		if idx < len(task.Cmds)-1 {
			fmt.Println()
		}
	}

	return nil
}

func taskExecCmd(ocmd, curtaskName string, gvars, vars map[string]string) error {
	cmd := os.Expand(ocmd, func(key string) string {
		if value, ok := vars[key]; ok {
			return value
		}
		if value, ok := gvars[key]; ok {
			return value
		}
		return key
	})
	cmdExec := newBashCmd(cmd)
	if err := cmdExec.Run(); err != nil {
		return err
	}
	return nil
}

type Cmd struct {
	bin  string
	args []string
	envs map[string]string
}

func (c *Cmd) Run() error {
	cmd := exec.Command(c.bin, c.args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	for key, value := range c.envs {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}
	return cmd.Run()
}

func newCmd(bin string, args ...string) *Cmd {
	return &Cmd{
		bin:  bin,
		args: args,
		envs: map[string]string{},
	}
}

func newSplitCmd(cmd string) *Cmd {
	argv := strings.Split(cmd, " ")
	return newCmd(argv[0], argv[1:]...)
}

func newBashCmd(cmd string) *Cmd {
	return newCmd("bash", "-c", cmd)
}

func (c *Cmd) withEnvs(envs map[string]string) *Cmd {
	c.envs = envs
	return c
}
