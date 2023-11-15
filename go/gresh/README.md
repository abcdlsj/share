# Gresh

`Gresh` is a CLI tool that can be used to execute commands automatically when `files` are `changed`.

## Usage

**Simple**

```shell
gresh <your command>
```

**More options**
```shell
./gresh -p <path> -c <command> -e <exclude pattern> -i <interval seconds>

# Like this
gresh -p 'test' -c 'make' -e '.md' -i 20
```