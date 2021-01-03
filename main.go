package main

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"
)

type Template struct {
	TemplateString string
	AllVars        []string
	LoopVars       []string
	Context        map[string]interface{}
}

func pop(alist *[]string) string {
	f := len(*alist)
	rv := (*alist)[f-1]
	*alist = append((*alist)[:f-1])
	return rv
}

func (t *Template) extractTokens() []string {
	re := regexp.MustCompile("(?s)({{.*?}}|{%.*?%}|{#.*?#})")

	toks := re.FindAllStringIndex(t.TemplateString, -1)
	var allTokenIndexes [][]int

	if toks[0][0] != 0 {
		allTokenIndexes = append(allTokenIndexes, []int{0, toks[0][0]})
	}

	for i := 0; i < len(toks); i++ {
		allTokenIndexes = append(allTokenIndexes, toks[i])
		if i == len(toks)-1 {
			if len(t.TemplateString) > toks[i][1] {
				allTokenIndexes = append(allTokenIndexes, []int{toks[i][1], len(t.TemplateString)})
			}
			break
		}

		allTokenIndexes = append(allTokenIndexes, []int{toks[i][1], toks[i+1][0]})
	}

	var allTokens []string
	for j := 0; j < len(allTokenIndexes); j++ {
		allTokens = append(allTokens, string(t.TemplateString[allTokenIndexes[j][0]:allTokenIndexes[j][1]]))
	}

	return allTokens
}

// ============ ==== ======= ==============
// ============ Code Builder ==============
// ============ ==== ======= ==============

type CodeBuilder struct {
	IndentLevel int
	Output      []interface{}
}

func (c *CodeBuilder) AddLine(text string) {
	currentLine := ""
	for i := 0; i < c.IndentLevel; i++ {
		currentLine += "\t"
	}
	currentLine += text + "\n"
	c.Output = append(c.Output, currentLine)
}

func (c *CodeBuilder) Indent() {
	c.IndentLevel += 1
}

func (c *CodeBuilder) Dedent() {
	c.IndentLevel -= 1
	c.AddLine("}\n")
}

func (c *CodeBuilder) AddSection() *interface{} {
	section := &CodeBuilder{
		IndentLevel: c.IndentLevel,
	}

	c.Output = append(c.Output, section)
	return &c.Output[len(c.Output)-1]
}

// ============ ==== ======= ==============
// ============ ==== ======= ==============

func (t *Template) ExprCode(expr string) string {
	code := ""
	if strings.Contains(expr, "|") {

	} else if strings.Contains(expr, ".") {
		dots := strings.Split(expr, ".")
		code = t.ExprCode(dots[0])
		temp := []string{}
		for i := 1; i < len(dots); i++ {
			temp = append(temp, fmt.Sprintf("%#v", dots[i]))
		}
		args := strings.Join(temp, ", ")
		code = "do_dots(" + code + "," + args + ")"
	} else {
		t.Variable(expr, &t.AllVars)
		code = "c_" + expr
	}
	return code
}

func (t *Template) Variable(name string, varsSet *[]string) {
	*varsSet = append(*varsSet, name)
}

func (t *Template) Compile() {
	code := &CodeBuilder{
		IndentLevel: 0,
	}

	code.AddLine("func renderFunction(context map[string]interface{}) string {")
	code.Indent()

	code.AddLine("result := []string{}")

	varsCode := code.AddSection()

	buffered := []string{}
	opStack := []string{}

	flushOutput := func() {
		if len(buffered) == 1 {
			code.AddLine("result = append(result, " + buffered[0] + ")")
		} else if len(buffered) > 1 {
			for i := 0; i < len(buffered)-1; i++ {
				if len(buffered[i]) > 0 {
					code.AddLine("result = append(result, " + buffered[i] + ")")
				}

			}
		}
		buffered = []string{}
	}

	tokens := t.extractTokens()
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]

		if token[:2] == "{#" {
			continue
		} else if token[:2] == "{%" {
			flushOutput()

			token = strings.TrimSpace(token[2 : len(token)-2])
			words := strings.Split(token, " ")

			if words[0] == "for" {
				opStack = append(opStack, "for")

				t.Variable("c_"+words[1], &t.LoopVars)

				temp := t.ExprCode(words[3])

				code.AddLine("for i := 0; i < len(" + temp + "); i++ {")
				code.Indent()
				code.AddLine("c_" + words[1] + " := " + temp + "[i]")

			} else if words[0] == "if" {
				opStack = append(opStack, "if")

			} else if words[0][:3] == "end" {
				if words[0][3:] == pop(&opStack) {
					code.Dedent()
				} else {
					fmt.Println("Mismatched end tag : " + words[0][:3])
				}
			}

		} else if token[:2] == "{{" {
			token = strings.TrimSpace(token[2 : len(token)-2])
			if len(token) > 0 {
				expr := t.ExprCode(token)
				buffered = append(buffered, expr)
			}
		} else {
			if len(token) > 0 {
				buffered = append(buffered, fmt.Sprintf("%#v", token))
			}
		}
	}

	for i := 0; i < len(t.AllVars)-1; i++ {
		(*varsCode).(*CodeBuilder).AddLine("c_" + t.AllVars[i] + " := context['" + t.AllVars[i] + "']")
	}

	flushOutput()

	code.AddLine("return strings.Join(result)")
	code.Dedent()

	fmt.Println()
	for i := 0; i < len(code.Output); i++ {
		switch code.Output[i].(type) {
		case string:
			fmt.Print(code.Output[i])
			break
		default:
			var temp *CodeBuilder
			temp = code.Output[i].(*CodeBuilder)
			for j := 0; j <= len(temp.Output)-1; j++ {
				fmt.Println(temp.Output[j])
			}
			break
		}
	}
}

func main() {
	var t Template
	data, err := ioutil.ReadFile("../pages/example.md")
	if err != nil {
		fmt.Println(err)
		return
	}

	t.TemplateString = string(data)
	t.Context = map[string]interface{}{
		"product_list": map[string]interface{}{
			"name":  "HDMI Cable",
			"price": 100,
		},
	}
	t.Compile()
}
