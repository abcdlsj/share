# Taky

`Go`'s command execute tools.

```
Usage of taky:
  -binary string
        Path to the binary to execute, use to generate service file
  -c string
        Path to the YAML config file
  -d    Run in daemon mode, will schedule all cron tasks to run
  -g    Use default config file
  -install
        Install the service file
  -l    List all tasks
  -t string
        Name of the task to execute
```

## Config

- current dir config file: `.taky.yaml`
- global default config file, located at `$HOME/.config/taky/taky.yaml`

if you add `-g` flag, it will use global default config file, if without `-g` and `-c` flag, it will use current dir config file first.
if there is no current dir config file, it will try to use global default config file.

## Example

```yaml
vars:
  OS: $(uname -s)
  ARCH: $(uname -m)
tasks:
  printos:
    cmds:
      - echo ${OS} ${ARCH}
  print-goenv:
    pres:
      - printos
    vars:
      GOPATH: $(go env GOPATH)
      GOBIN: $(go env GOROOT)
    cmds:
      - echo GOPATH ${GOPATH}
      - echo GOROOT ${GOROOT}
  cron-printos:
    cron: "*/1 * * * *"
    cmds:
      - echo ${OS} ${ARCH}
```

`vars`: variables
`tasks`: tasks
    - `vars`: command variables
    - `cmds`: commands, support multiple commands
    - `pres`: pre command, will run before commands

## Background

`Taky` can run at background and will schedule to execute `all` cron tasks.

Can use `sudo taky -install -binary <BINARY> -c <CONFIG>` to install service file.

### Darwin

It will install at `/Library/LaunchDaemons/com.abcdlsj.taky.plist`
You can run `sudo launchctl load /Library/LaunchDaemons/com.abcdlsj.taky.plist` to start service.

### Linux

`Comming soon`
