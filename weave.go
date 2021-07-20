package main

import (
	"fmt"
	"go/token"
	"go/types"
	"io/ioutil"
	"log"
	"regexp"
	"strings"
)

func check(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

type Node struct {
	children 	[]Node
	nodeType 	string
	varName  	string
	text		string
	data		string
	condition	string
}

func readFile(filename string) string {
	res, err := ioutil.ReadFile(filename)
	check(err)
	return string(res)
}

func extractTokens(templateString string) []string {
	re := regexp.MustCompile("(?s)({{.*?}}|{%.*?%}|{#.*?#}|{!.*?})")

	tokens := re.FindAllStringIndex(templateString, -1)
	var allTokenIndexes [][]int

	if tokens[0][0] != 0 {
		allTokenIndexes = append(allTokenIndexes, []int{0, tokens[0][0]})
	}

	for i := 0; i < len(tokens); i++ {
		allTokenIndexes = append(allTokenIndexes, tokens[i])
		if i == len(tokens)-1 {
			if len(templateString) > tokens[i][1] {
				allTokenIndexes = append(allTokenIndexes, []int{tokens[i][1], len(templateString)})
			}
			break
		}
		allTokenIndexes = append(allTokenIndexes, []int{tokens[i][1], tokens[i+1][0]})
	}

	var allTokens []string
	for j := 0; j < len(allTokenIndexes); j++ {
		allTokens = append(allTokens, string(templateString[allTokenIndexes[j][0]:allTokenIndexes[j][1]]))
	}

	return allTokens
}

func constructSyntaxTree(template string) Node {
	var tokens = extractTokens(template)

	var stack []Node
	stack = append(stack, Node{nodeType: "root"})

	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if strings.HasPrefix(token, "{{") && strings.HasSuffix(token, "}}") {
			varName := strings.TrimSpace(strings.TrimRight(strings.TrimLeft(token, "{{"), "}}"))
			varNode := Node{nodeType: "var", varName: varName}
			stack[len(stack)-1].children = append(stack[len(stack)-1].children, varNode)
		} else if strings.HasPrefix(token, "{") && strings.HasSuffix(token, "}") {
			words := strings.Split(strings.TrimSpace(strings.TrimRight(strings.TrimLeft(token, "{%"), "%}")), " ")

			if words[0] == "end" && len(stack) > 0 {
				stack[len(stack)-2].children = append(stack[len(stack)-2].children, stack[len(stack)-1])
				stack = stack[:len(stack)-1]
			}

			if words[0] == "each" {
				eachNode := Node{nodeType: "each", varName: words[1]}
				stack = append(stack, eachNode)
			}

			if words[0] == "for" {
				forNode := Node{nodeType: "for", varName: words[1], data: words[3]}
				stack = append(stack, forNode)
			}

			if words[0] == "if" {
				ifNode := Node{nodeType: "if", condition: strings.Join(words[1:], " ")}
				stack = append(stack, ifNode)
			}

			if words[0] == "else" {
				if len(stack) == 0 {
					log.Fatalln("incorrect syntax, else with no if")
				}

				ifNode := Node{nodeType: "if", condition: "!(" + stack[len(stack)-1].condition + ")"}
				stack[len(stack)-2].children = append(stack[len(stack)-2].children, stack[len(stack)-1])
				stack = stack[:len(stack)-1]
				stack = append(stack, ifNode)
			}
		} else {
			textNode := Node{nodeType: "text", text: token}
			stack[len(stack)-1].children = append(stack[len(stack)-1].children, textNode)
		}
	}
	return stack[0]
}

func (n Node) Print(indent int, c map[string]interface{}) string {
	res := ""
	for _, child := range n.children {
		if child.nodeType == "var" {
			res += fmt.Sprintf("%v", c[child.varName])
		}

		if child.nodeType == "text" {
			res += child.text
		}

		if child.nodeType == "each" {
			tempContext := c
			iter := c[child.varName].([]int)
			for i := 0; i < len(iter); i++ {
				tempContext["it"] = iter[i]
				res += child.Print(indent+1, tempContext)
			}
		}

		if child.nodeType == "for" {
			tempContext := c
			iter := c[child.data].([]int)
			for i := 0; i < len(iter); i++ {
				tempContext[child.varName] = iter[i]
				res += child.Print(indent+1, tempContext)
			}
		}

		if len(child.children) > 0 {
			if child.nodeType == "if" {
				if calc(child.condition) == "false" {
					continue
				}
			}
			if child.nodeType == "text" {
				res += child.Print(indent+1, c)
			}
		}
	}
	return res
}

func calc(c string) string {
	fs := token.NewFileSet()
	tv, err := types.Eval(fs, nil, token.NoPos, c)
	check(err)
	return tv.Value.String()
}
