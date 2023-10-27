package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"text/template"

	"github.com/abcdlsj/crone"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Vars  map[string]string `yaml:"vars"`
	Tasks map[string]Task   `yaml:"tasks"`
}

type Task struct {
	Cron string            `yaml:"cron"`
	Pres []string          `yaml:"pres"`
	Cmds []string          `yaml:"cmds"`
	Vars map[string]string `yaml:"vars"`
}

var (
	taskName string
	cfgFile  string
	binFile  string

	daemon     bool
	install    bool
	defaultCfg bool
	listTasks  bool

	homeDir, _ = os.UserHomeDir()

	deftCfgFile = filepath.Join(homeDir, ".config/taky/config.yaml")

	cfg Config

	//go:embed com.abcdlsj.taky.plist
	serviceTmpls embed.FS
)

func main() {
	flag.StringVar(&taskName, "t", "", "Name of the task to execute")
	flag.StringVar(&cfgFile, "c", "", "Path to the YAML config file")
	flag.StringVar(&binFile, "binary", "", "Path to the binary to execute, use to generate service file")
	flag.BoolVar(&defaultCfg, "g", false, "Use default config file")
	flag.BoolVar(&install, "install", false, "Install the service file")
	flag.BoolVar(&listTasks, "l", false, "List all tasks")
	flag.BoolVar(&daemon, "d", false, "Run in daemon mode, will schedule all cron tasks to run")

	flag.Parse()

	if len(os.Args) == 1 {
		flag.Usage()
		return
	}

	if install {
		installService()
		return
	}

	if defaultCfg {
		cfgFile = deftCfgFile
	}

	if cfgFile == "" {
		if _, err := os.Stat(".taky.yaml"); err == nil {
			cfgFile = ".taky.yaml"
		} else {
			if _, err := os.Stat(deftCfgFile); err == nil {
				cfgFile = deftCfgFile
			}
		}
	}

	cfgData, err := os.ReadFile(cfgFile)
	if err != nil {
		fmt.Printf("Failed to read config file, %v\n", err)
		return
	}

	err = yaml.Unmarshal(cfgData, &cfg)
	if err != nil {
		fmt.Printf("Failed to parse config file, %v\n", err)
		return
	}

	fmt.Printf("Loaded config file: %s\n", cfgFile)

	cfgVars := map[string]string{}
	for key, value := range cfg.Vars {
		cfgVars[key] = os.ExpandEnv(value)
	}

	if listTasks {
		format := "%-15s: %s\n"
		for name := range cfg.Tasks {
			fmt.Printf(format, name, cfg.Tasks[name].Cmds[0])
		}
		return
	}

	if !daemon && taskName == "" {
		fmt.Println("Please specify a task name")
		return
	}

	if taskName != "" {
		task, ok := cfg.Tasks[taskName]
		if !ok {
			fmt.Printf("Task %s not found\n", taskName)
			return
		}

		if err := taskExec(task, taskName, cfgVars); err != nil {
			fmt.Printf("Failed to execute task, %v\n", err)
			return
		}
	}

	if daemon {
		schedule(cfg)
	}
}

func schedule(cfg Config) {
	sr := crone.NewSchduler()
	for name, task := range cfg.Tasks {
		if task.Cron != "" {
			sr.Add(name, task.Cron, func() {
				taskExec(task, name, cfg.Vars)
			})
		}
	}

	sr.StartWithSignalListen()

	sr.Wait()
}

func taskExec(task Task, taskName string, cfgVars map[string]string) error {
	vars := map[string]string{}
	for key, value := range task.Vars {
		vars[key] = os.ExpandEnv(value)
	}

	for _, pre := range task.Pres {
		_, ok := cfg.Tasks[pre]
		if !ok {
			fmt.Printf("Task %s not found\n", pre)
			return errors.New("task not found")
		}

		if err := taskExec(cfg.Tasks[pre], pre, cfgVars); err != nil {
			return err
		}
	}

	for idx, cmd := range task.Cmds {
		if err := taskExecCmd(cmd, taskName, cfgVars, vars); err != nil {
			log.Fatalf("Failed to execute command: %v", err)
		}

		if idx < len(task.Cmds)-1 {
			fmt.Println()
		}
	}

	return nil
}

var deftCfgFileContent = `
vars:
  OS: $(uname -s)
  ARCH: $(uname -m)
tasks:
  hello:
    cmds:
      - echo hello ${OS} ${ARCH}
`

func installService() {
	if os.Geteuid() != 0 {
		fmt.Println("please run as sudo")
		return
	}

	if binFile == "" {
		log.Fatal("bin file not found")
	}
	if cfgFile == "" {
		log.Fatal("config file not found")
	}

	if _, err := os.Stat(cfgFile); err != nil {
		if err := os.MkdirAll(filepath.Dir(cfgFile), 0755); err != nil {
			log.Fatal(err)
		}
		cfg, err := os.Create(cfgFile)
		if err != nil {
			log.Fatal(err)
		}
		cfg.WriteString(deftCfgFileContent)
		cfg.Close()
		log.Printf("created config file: %s\n", cfgFile)
	}

	switch runtime.GOOS {
	case "darwin":
		tmpl := template.Must(template.ParseFS(serviceTmpls, "com.abcdlsj.taky.plist"))
		if _, err := os.Stat("/Library/LaunchDaemons/com.abcdlsj.taky.plist"); err != nil {
			file, err := os.OpenFile("/Library/LaunchDaemons/com.abcdlsj.taky.plist", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
			if err != nil {
				log.Fatal(err)
			}

			defer file.Close()

			err = tmpl.Execute(file, struct {
				HomeDir string
				BinFile string
				CfgFile string
			}{
				HomeDir: homeDir,
				BinFile: binFile,
				CfgFile: cfgFile,
			})

			if err != nil {
				log.Fatal(err)
			}
		}

		log.Printf("created service file: %s\n", "/Library/LaunchDaemons/com.abcdlsj.taky.plist")
		fmt.Printf("run `sudo launchctl load /Library/LaunchDaemons/com.abcdlsj.taky.plist` to enable the service\n")
	}
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

func newBashCmd(cmd string) *Cmd {
	return newCmd("bash", "-c", cmd)
}
