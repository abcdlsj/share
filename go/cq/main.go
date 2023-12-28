package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		fmt.Print("Usage: cq '[condition]' [file]\n")
		fmt.Print("       cat file | cq '[condition]'\n")

		return
	}

	cq := CQ{
		input:  os.Stdin,
		output: os.Stdout,
	}

	if len(args) == 2 {
		var multir io.Reader

		f, err := os.Open(args[1])
		if err != nil {
			log.Fatalf("Failed to open file: %s", err)
		}

		if multir == nil {
			multir = f
		}

		cq.input = multir
	}

	cons, isexclude := parseCondition(args[0])

	if isexclude {
		cq.queryExcludeCols = cons
	} else {
		cq.queryCols = cons
	}

	cq.Run()
}

func parseCondition(s string) ([]string, bool) {
	isexclude := false

	if strings.HasPrefix(s, "!") {
		s = s[1:]
		isexclude = true
	}

	if strings.HasPrefix(s, "[") {
		s = s[1 : len(s)-1]
	}

	return strings.Split(s, ","), isexclude
}

type CQ struct {
	input            io.Reader
	output           io.Writer
	queryCols        []string
	queryExcludeCols []string
}

type CSVData struct {
	hdrcols []string
	rows    map[string][]string
}

func (cq *CQ) Run() {
	data, err := io.ReadAll(cq.input)
	if err != nil {
		log.Fatalf("Failed to read input: %s", err)
	}

	csv := parseCSV(data)

	cols := buildCols(cq.queryCols, cq.queryExcludeCols, csv.hdrcols)

	if len(cols) == 0 {
		return
	}

	cq.Write([]byte(strings.Join(cols, ",")))

	for lineidx := 0; lineidx < len(csv.rows[cols[0]]); lineidx++ {
		cq.Write([]byte("\n"))

		roweles := make([]string, 0, len(cols))
		for _, col := range cols {
			if lineidx > len(csv.rows[col]) {
				continue
			}
			roweles = append(roweles, csv.rows[col][lineidx])
		}

		cq.Write([]byte(strings.Join(roweles, ",")))
	}

	cq.Write([]byte("\n"))
}

func buildCols(qcols, excols, allcols []string) []string {
	diff := difference(allcols, excols)

	if len(qcols) == 0 {
		return diff
	}

	result := intersection(qcols, diff)

	return result
}

func difference(a, b []string) []string {
	mb := make(map[string]bool, len(b))
	for _, x := range b {
		mb[x] = true
	}
	var diff []string
	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}

func intersection(a, b []string) []string {
	mb := make(map[string]bool, len(b))
	for _, x := range b {
		mb[x] = true
	}
	var insec []string
	for _, x := range a {
		if _, found := mb[x]; found {
			insec = append(insec, x)
		}
	}
	return insec
}

func parseCSV(content []byte) *CSVData {
	lines := strings.Split(string(content), "\n")

	csvdata := &CSVData{
		hdrcols: strings.Split(lines[0], ","),
		rows:    make(map[string][]string),
	}

	for i := 1; i < len(lines); i++ {
		splits := strings.Split(lines[i], ",")
		for j := 0; j < len(splits); j++ {
			csvdata.rows[csvdata.hdrcols[j]] = append(csvdata.rows[csvdata.hdrcols[j]], splits[j])
		}
	}

	return csvdata
}

func (p *CQ) Write(content []byte) {
	p.output.Write(content)
}
