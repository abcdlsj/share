package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		panic("please input repo path")
	}
	walk(args[0])
}

func walk(dir string) {
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		fmt.Printf("walk at: %s\n", path)
		if d.IsDir() || !checkFile(d.Name()) {
			return nil
		}
		input, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lines := strings.Split(string(input), "\n")
		for i, line := range lines {
			nLine := mask(line)
			if line != nLine {
				lines[i] = nLine
			}
		}
		output := strings.Join(lines, "\n")
		return os.WriteFile(path, []byte(output), d.Type().Perm())
	})
}

// MASK: `RULE_TYPE` `RULE` `REPLACE_STR`
func mask(line string) string {
	cline := line
	if strings.Contains(line, "// MASK:") && !strings.HasPrefix(line, "//") && !strings.HasPrefix(line, "/*") {
		ruleIndex := strings.LastIndex(line, "// MASK:")
		codeStr, ruleStr := line[:ruleIndex], line[ruleIndex+9:]
		fmt.Printf("rule_str:%s\n", ruleStr)
		if !strings.Contains(ruleStr, "`match`") && !strings.Contains(ruleStr, "`regexp`") {
			panic("not support rule type")
		}
		ruleType := strings.Split(ruleStr, "`")[1]
		rule := strings.Split(ruleStr, "`")[3]
		replaceStr := strings.Split(ruleStr, "`")[5]
		fmt.Printf("rule_type:%s, rule:%s, replace_str:%s\n", ruleType, rule, replaceStr)
		switch ruleType {
		case "match":
			codeStr = strings.ReplaceAll(codeStr, rule, replaceStr)
		case "regexp":
			codeStr = regexp.MustCompile(rule).ReplaceAllString(codeStr, replaceStr)
		}
		line = codeStr + "// MASK_DONE"
		fmt.Printf("replace result:\n \tsource: %s\n \tdest: %s\n\n", cline, line)
	}
	return line
}

func checkFile(name string) bool {
	return strings.HasSuffix(name, ".go") && !strings.HasPrefix(name, ".")
}
