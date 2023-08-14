package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/abcdlsj/cr"
)

var (
	dir          = "."
	search       = ""
	excludeFlags arrayFlags
)

type arrayFlags []string

func (i *arrayFlags) String() string {
	return "string representation"
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func init() {
	flag.StringVar(&dir, "d", ".", "directory to search")
	flag.StringVar(&search, "s", "", "search string")
	flag.Var(&excludeFlags, "e", "exclude directory")
}

func main() {
	flag.Parse()

	if search == "" {
		fmt.Println("search string is empty")
		return
	}

	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !d.Type().IsRegular() || isExclude(path) {
			return nil
		}

		return scan(path)
	})
}

func isExclude(path string) bool {
	for _, exclude := range excludeFlags {
		if strings.Contains(path, exclude) {
			return true
		}
	}

	return false
}

func scan(path string) error {
	f, err := os.Open(path)

	scanner := bufio.NewScanner(f)
	if err != nil {
		return err
	}
	defer f.Close()

	i := 0
	pfile := false
	for scanner.Scan() {
		if isBinary(scanner.Bytes()) {
			return nil
		}
		if bytes.Contains(scanner.Bytes(), []byte(search)) {
			if !pfile {
				fmt.Printf("%s\n", cr.PLBlue(path))
				pfile = true
			}
			fmt.Printf("%s: %s\n", cr.PLGreen(strconv.Itoa(i+1)), redContain(scanner.Text(), string(search)))
		}
		i++
	}

	return nil
}

func isBinary(buffer []byte) bool {
	for _, b := range buffer {
		if b == 0 || (b < 32 && b != 9 && b != 10 && b != 13) {
			return true
		}
	}

	return false
}

func redContain(s, sub string) string {
	return strings.ReplaceAll(s, sub, cr.PLRedBold(sub))
}
