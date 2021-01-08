package engine

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"regexp"
	"strings"
)

type Template struct {
	TemplateString string
	AllVars        []string
	LoopVars       []string
	Context        map[string]interface{}
}

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

func pop(alist *[]string) string {
	listLength := len(*alist)
	poppedValue := (*alist)[listLength-1]
	*alist = (*alist)[:listLength-1]
	return poppedValue
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

func (t *Template) ExprCode(expr string) string {
	code := ""
	if strings.Contains(expr, "|") {

	} else if strings.Contains(expr, ".") {
		dots := strings.Split(expr, ".")
		code = t.ExprCode(dots[0])

		x := code
		code = "("

		for i := 0; i < len(dots)-2; i++ {
			code += "("
		}

		code += x + ").(map[string]interface{})"

		for i := 0; i < len(dots)-2; i++ {
			code += "[" + fmt.Sprintf("%#v", dots[1:][i]) + "]"
			if i+1 != len(dots)-1 {
				code += ").(map[string]interface{})"
			}
		}

		code = "reflect.ValueOf(" + code + ").MapIndex(reflect.ValueOf(\"" + dots[len(dots)-1] + "\")).Interface()"

	} else {
		t.Variable(expr, &t.AllVars)
		code = "c_" + expr
	}
	return code
}

func (t *Template) Variable(name string, varsSet *[]string) {
	*varsSet = append(*varsSet, name)
}

func (t *Template) Compile(outputFunctionName string) CodeBuilder {
	code := &CodeBuilder{
		IndentLevel: 0,
	}

	contextInJson, err := json.Marshal(t.Context)
	if err != nil {
		fmt.Println(err)
	}

	code.AddLine("func " + outputFunctionName + "() string {")
	code.Indent()

	code.AddLine("var context map[string]interface{}")
	code.AddLine("if err := json.Unmarshal([]byte(" + fmt.Sprintf("%#v", string(contextInJson)) + "), &context); err != nil {")
	code.Indent()
	code.AddLine("return \"\"")
	code.Dedent()

	code.AddLine("result := []string{}")

	varsCode := code.AddSection()

	buffered := []string{}
	opStack := []string{}

	flushOutput := func() {
		for i := 0; i < len(buffered); i++ {
			if len(buffered[i]) > 0 {
				if buffered[i][len(buffered[i])-12:] == ".Interface()" {
					buffered[i] += ".(string)"
				}
				code.AddLine("result = append(result, " + buffered[i] + ")")
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

				code.AddLine("for i := 0; i < len(" + temp + ".([]interface{})); i++ {")
				code.Indent()
				code.AddLine("c_" + words[1] + " := " + temp + ".([]interface{})[i]")

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
			buffered = append(buffered, fmt.Sprintf("%#v", token))
		}
	}

	contains := func(s []string, e string) bool {
		for _, a := range s {
			if a == e {
				return true
			}
		}
		return false
	}

	var done []string
	for i := 0; i < len(t.AllVars); i++ {
		if !contains(t.LoopVars, "c_"+t.AllVars[i]) && !contains(done, "c_"+t.AllVars[i]) {
			(*varsCode).(*CodeBuilder).AddLine("c_" + t.AllVars[i] + " := context[\"" + t.AllVars[i] + "\"]")
			done = append(done, "c_"+t.AllVars[i])
		}
	}

	flushOutput()

	code.AddLine("return strings.Join(result, \"\")")
	code.Dedent()

	return *code
}

func (t *Template) Render(outputFunctionName string) string {
	code := t.Compile(outputFunctionName)

	output := ""

	for i := 0; i < len(code.Output); i++ {
		switch code.Output[i].(type) {
		case string:
			output += code.Output[i].(string)
		default:
			var temp *CodeBuilder
			temp = code.Output[i].(*CodeBuilder)
			for j := 0; j <= len(temp.Output)-1; j++ {
				output += (temp.Output[j]).(string) + "\n"
			}
		}
	}

	return output
}

func RenderTemplateString(templateString, renderFunctionName string, context map[string]interface{}) string {
	t := Template{
		Context:        context,
		TemplateString: templateString,
	}
	return t.Render("c_render_" + renderFunctionName)
}

func RenderFile(filename string, context map[string]interface{}) string {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatal(err)
		return ""
	}

	temp := strings.Split(filename, "/")
	renderFunctionName := "c_render_" + strings.Replace(temp[len(temp)-1], ".", "_", -1)
	return RenderTemplateString(string(data), renderFunctionName, context)
}
