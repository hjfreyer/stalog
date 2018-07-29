package main

import (
	"log"

	"github.com/hjfreyer/stalog/parser"
)

func main() {
	ast := parser.StalogAST{Buffer: `
# Check
package   foo

symbol Z
symbol S
        `}

	ast.Init()
	if err := ast.Parse(); err != nil {
		log.Fatal(err)
	}
	ast.PrintSyntaxTree()
}
