package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
)

type Counter struct {
	idx              int
	lang             string
	comment          string
	multiLineComment []string
	exts             []string
}

var (
	Go       = Counter{1, "Go", "//", vec("/*", "*/"), vec(".go")}
	Rust     = Counter{2, "Rust", "//", nil, vec(".rs")}
	Java     = Counter{3, "Java", "//", nil, vec(".java")}
	Python   = Counter{4, "Python", "#", nil, vec(".py")}
	C        = Counter{5, "C", "//", nil, vec(".c", ".h")}
	Cpp      = Counter{6, "C++", "//", nil, vec(".cpp", ".hpp")}
	Js       = Counter{7, "Javascript", "//", nil, vec(".js")}
	Ts       = Counter{8, "Typescript", "//", nil, vec(".ts")}
	HTML     = Counter{9, "HTML", "//", nil, vec(".html", ".htm")}
	JSON     = Counter{10, "JSON", "//", nil, vec(".json")}
	Protobuf = Counter{11, "Protobuf", "//", nil, vec(".proto")}
	Markdown = Counter{12, "Markdown", "//", nil, vec(".md")}
	Shell    = Counter{13, "Shell", "#", nil, vec(".sh")}
	YAML     = Counter{14, "YAML", "#", nil, vec(".yaml", ".yml")}
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

	result = &Result{
		data: make([]Item, registedNum),
	}
}

var result *Result

var fileChan = make(chan string, 100)

func process(dir string) {
	var wg sync.WaitGroup
	wg.Add(runtime.NumCPU() * 2)

	for i := 0; i < runtime.NumCPU()*2; i++ {
		go func() {
			defer wg.Done()
			for file := range fileChan {
				countLine(file)
			}
		}()
	}

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			panic(err)
		}

		if info.IsDir() {
			return nil
		}

		fileChan <- path

		return nil
	})

	close(fileChan)

	wg.Wait()
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: tally <path>")
		os.Exit(1)
	}

	process(os.Args[1])

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
	mu   sync.Mutex
	data []Item
}

func (r *Result) Add(c Counter, item Item) {
	r.mu.Lock()
	defer r.mu.Unlock()
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

func (c Counter) isMultiLineComment(s []byte) int {
	if c.multiLineComment == nil {
		return 0
	}

	if bytes.HasPrefix(s, []byte(c.multiLineComment[0])) {
		return -1
	}

	if bytes.HasPrefix(s, []byte(c.multiLineComment[1])) {
		return 1
	}

	return 0
}

func (c Counter) toMultiLineCommentEnd(scanner *bufio.Scanner) int {
	var i int
	for scanner.Scan() {
		if n := c.isMultiLineComment(scanner.Bytes()); n != 1 {
			i++
			continue
		}
		return i
	}
	return i
}

func guessLang(file string) Counter {
	return ext2Counter[filepath.Ext(file)]
}

func countLine(path string) error {
	f, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(bytes.NewReader(f))

	if err != nil {
		return err
	}

	c := guessLang(path)
	if c.lang == "" {
		return nil
	}

	item := Item{
		lang:  c.lang,
		files: 1,
	}

	for scanner.Scan() {
		if isBinary(scanner.Bytes()) {
			return nil
		}

		item.lines++
		line := bytes.TrimSpace(scanner.Bytes())

		if len(line) == 0 {
			item.blank++
			continue
		}

		if c.isComment(line) {
			item.comment++
			continue
		} else if n := c.isMultiLineComment(line); n == -1 {
			l := c.toMultiLineCommentEnd(scanner)
			item.comment += l + 2
			item.lines += l + 1
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
