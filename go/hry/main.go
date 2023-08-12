package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"time"
)

var (
	file      string
	mstr      string
	cmd       string
	mergeFrom string
	clear     bool
	apend     bool

	HOMEDIR = os.Getenv("HOME")
)

func init() {
	flag.StringVar(&file, "f", path.Join(HOMEDIR, ".config/fish/history.hry"), "fish history file")
	flag.StringVar(&mstr, "s", "", "match string")
	flag.StringVar(&cmd, "c", "", "command to run")
	flag.StringVar(&mergeFrom, "m", "", "merge from history file")
	flag.BoolVar(&clear, "clear", false, "clear history")
	flag.BoolVar(&apend, "a", false, "append to history")
}

func main() {
	flag.Parse()

	if clear {
		clearHry(file)
		return
	}

	if mergeFrom != "" {
		mergeHry(file, mergeFrom)
		return
	}

	if apend {
		all, err := parseHryFile(file)
		if err != nil {
			panic(err)
		}

		pwd, _ := os.Getwd()
		for _, r := range all {
			if r.cmd == cmd && r.pwd == pwd {
				return
			}
		}

		when := time.Now().Unix()
		appendItem(format(cmd, pwd, when))
		return
	}

	if mstr != "" {
		results, err := matchItem(mstr)
		if err != nil {
			panic(err)
		}

		for _, r := range results {
			fmt.Printf("%s\n", r.cmd)
		}
	} else {
		results, err := parseHryFile(file)
		if err != nil {
			panic(err)
		}

		sort.Slice(results, func(i, j int) bool {
			return results[i].when > results[j].when
		})

		for _, r := range results {
			fmt.Printf("%s\n", r.cmd)
		}
	}
}

type searchResult struct {
	cmd  string
	pwd  string
	when string
}

func matchItem(mstr string) ([]searchResult, error) {
	results, err := parseHryFile(file)
	if err != nil {
		return nil, err
	}

	pwd, _ := os.Getwd()
	var match1, match2 []searchResult
	for _, r := range results {
		if strings.HasPrefix(r.cmd, mstr) {
			if pwd == r.pwd {
				match1 = append(match1, r)
				continue
			}
			match2 = append(match2, r)
		}
	}

	sort.Slice(match1, func(i, j int) bool {
		return match1[i].when > match1[j].when
	})
	sort.Slice(match2, func(i, j int) bool {
		return match2[i].when > match2[j].when
	})

	return append(match1, match2...), nil
}

func parseHryFile(file string) ([]searchResult, error) {
	f, err := os.OpenFile(file, os.O_RDONLY, 0600)
	if err != nil {
		return nil, err
	}

	var ret []searchResult

	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) != 3 {
			continue
		}

		ret = append(ret, searchResult{unescape(parts[0]), unescape(parts[1]), parts[2]})
	}

	return ret, nil
}

func appendItem(item string) {
	f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	if _, err = f.WriteString(item + "\n"); err != nil {
		panic(err)
	}
}

func clearHry(file string) {
	f, err := os.OpenFile(file, os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	defer f.Close()
}

func mergeHry(to, from string) {
	fromItems, err := parseHryFile(from)
	if err != nil {
		panic(err)
	}

	for _, item := range fromItems {
		appendItem(format(item.cmd, item.pwd, time.Now().Unix()))
	}

	clearHry(from)
}

func format(cmd, pwd string, when int64) string {
	return fmt.Sprintf("%s|%s|%d", escape(cmd), escape(pwd), when)
}

func escape(s string) string {
	return strings.Replace(s, "'", "\\'", -1)
}

func unescape(s string) string {
	return strings.Replace(s, "\\'", "'", -1)
}
