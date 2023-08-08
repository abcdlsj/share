package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Counter struct {
	idx     int
	lang    string
	comment string
	exts    []string
}

var (
	Go       = Counter{1, "Go", "//", vec(".go")}
	Rust     = Counter{2, "Rust", "//", vec(".rs")}
	Java     = Counter{3, "Java", "//", vec(".java")}
	Python   = Counter{4, "Python", "#", vec(".py")}
	C        = Counter{5, "C", "//", vec(".c", ".h")}
	Cpp      = Counter{6, "C++", "//", vec(".cpp", ".hpp")}
	Js       = Counter{7, "Javascript", "//", vec(".js")}
	Ts       = Counter{8, "Typescript", "//", vec(".ts")}
	HTML     = Counter{9, "HTML", "//", vec(".html", ".htm")}
	JSON     = Counter{10, "JSON", "//", vec(".json")}
	Protobuf = Counter{11, "Protobuf", "//", vec(".proto")}
	Markdown = Counter{12, "Markdown", "//", vec(".md")}
	Shell    = Counter{13, "Shell", "#", vec(".sh")}
	YAML     = Counter{14, "YAML", "#", vec(".yaml", ".yml")}
)

var ext2Counter = map[string]Counter{}
var registedNum = 0

func init() {
	for _, c := range []Counter{
		Go, Rust, Java, Python, C, Cpp, Js, Ts, HTML, JSON, Protobuf, Markdown, Shell, YAML,
	} {
		registedNum++
		for _, ext := range c.exts {
			ext2Counter[ext] = c
		}
	}

	result = NewResult()
}

var result *Result

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: tally <path>")
		os.Exit(1)
	}

	filepath.Walk(os.Args[1], func(path string, info os.FileInfo, err error) error {
		if err != nil {
			panic(err)
		}

		if info.IsDir() {
			return nil
		}

		return countLine(path)
	})

	result.String()
}

type Item struct {
	lang    string
	files   int
	lines   int
	code    int
	blank   int
	comment int
}

func mergeItem(a, b Item) Item {
	return Item{
		lang:    or(a.lang, b.lang),
		files:   a.files + b.files,
		lines:   a.lines + b.lines,
		code:    a.code + b.code,
		blank:   a.blank + b.blank,
		comment: a.comment + b.comment,
	}
}

type Result struct {
	data []Item
}

func NewResult() *Result {
	return &Result{
		data: make([]Item, registedNum),
	}
}

func (r *Result) Add(c Counter, item Item) {
	r.data[c.idx-1] = mergeItem(r.data[c.idx-1], item)
}

func (r *Result) String() {
	itemF := " %-10s %10d %10d %10d %10d %10d \n"
	headerF := " %-10s %10s %10s %10s %10s %10s \n"
	borderLen := 67
	fmt.Printf(strings.Repeat("━", borderLen) + "\n")
	fmt.Printf(headerF, "Language", "Files", "Lines", "Code", "Comments", "Blanks")
	fmt.Printf(strings.Repeat("━", borderLen) + "\n")

	var total Item

	sort.Slice(r.data, func(i, j int) bool {
		return r.data[i].lines > r.data[j].lines
	})
	for _, item := range r.data {
		if item.files == 0 {
			continue
		}

		total = mergeItem(total, item)
		fmt.Printf(itemF, item.lang, item.files, item.lines, item.code, item.comment, item.blank)
	}

	fmt.Printf(strings.Repeat("━", borderLen) + "\n")
	fmt.Printf(itemF, "Total", total.files, total.lines, total.code, total.comment, total.blank)
	fmt.Printf(strings.Repeat("━", borderLen) + "\n")
}

func (c Counter) isComment(s []byte) bool {
	return bytes.HasPrefix(s, []byte(c.comment))
}

func guessLang(file string) Counter {
	return ext2Counter[filepath.Ext(file)]
}

func countLine(path string) error {
	f, err := os.Open(path)
	scanner := bufio.NewScanner(f)
	if err != nil {
		return err
	}
	defer f.Close()

	c := guessLang(path)
	if c.lang == "" {
		return nil
	}

	item := Item{
		lang:  c.lang,
		files: 1,
	}

	for scanner.Scan() {
		item.lines++
		if isBinary(scanner.Bytes()) {
			return nil
		}
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			item.blank++
			continue
		}
		if c.isComment(line) {
			item.comment++
			continue
		}
		item.code++
	}

	result.Add(c, item)
	return nil
}

func vec(s ...string) []string {
	return s
}

func or(a, b string) string {
	if a == "" {
		return b
	}
	return a
}

func isBinary(buffer []byte) bool {
	for _, b := range buffer {
		if b == 0 || (b < 32 && b != 9 && b != 10 && b != 13) {
			return true
		}
	}

	return false
}
