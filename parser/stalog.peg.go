package parser

//go:generate peg go/src/github.com/hjfreyer/stalog/parser/stalog.peg

import (
	"fmt"
	"math"
	"sort"
	"strconv"
)

const endSymbol rune = 1114112

/* The rule types inferred from the grammar are below. */
type pegRule uint8

const (
	ruleUnknown pegRule = iota
	ruleModule
	ruleDefinition
	ruleSymbolDef
	ruleIdentifier
	ruleSymbolName
	ruleDefName
	ruleSpace
	ruleSpacing
	ruleWhiteSpace
	ruleComment
	ruleEndOfFile
	ruleEndOfLine
	rulePegText
)

var rul3s = [...]string{
	"Unknown",
	"Module",
	"Definition",
	"SymbolDef",
	"Identifier",
	"SymbolName",
	"DefName",
	"Space",
	"Spacing",
	"WhiteSpace",
	"Comment",
	"EndOfFile",
	"EndOfLine",
	"PegText",
}

type token32 struct {
	pegRule
	begin, end uint32
}

func (t *token32) String() string {
	return fmt.Sprintf("\x1B[34m%v\x1B[m %v %v", rul3s[t.pegRule], t.begin, t.end)
}

type node32 struct {
	token32
	up, next *node32
}

func (node *node32) print(pretty bool, buffer string) {
	var print func(node *node32, depth int)
	print = func(node *node32, depth int) {
		for node != nil {
			for c := 0; c < depth; c++ {
				fmt.Printf(" ")
			}
			rule := rul3s[node.pegRule]
			quote := strconv.Quote(string(([]rune(buffer)[node.begin:node.end])))
			if !pretty {
				fmt.Printf("%v %v\n", rule, quote)
			} else {
				fmt.Printf("\x1B[34m%v\x1B[m %v\n", rule, quote)
			}
			if node.up != nil {
				print(node.up, depth+1)
			}
			node = node.next
		}
	}
	print(node, 0)
}

func (node *node32) Print(buffer string) {
	node.print(false, buffer)
}

func (node *node32) PrettyPrint(buffer string) {
	node.print(true, buffer)
}

type tokens32 struct {
	tree []token32
}

func (t *tokens32) Trim(length uint32) {
	t.tree = t.tree[:length]
}

func (t *tokens32) Print() {
	for _, token := range t.tree {
		fmt.Println(token.String())
	}
}

func (t *tokens32) AST() *node32 {
	type element struct {
		node *node32
		down *element
	}
	tokens := t.Tokens()
	var stack *element
	for _, token := range tokens {
		if token.begin == token.end {
			continue
		}
		node := &node32{token32: token}
		for stack != nil && stack.node.begin >= token.begin && stack.node.end <= token.end {
			stack.node.next = node.up
			node.up = stack.node
			stack = stack.down
		}
		stack = &element{node: node, down: stack}
	}
	if stack != nil {
		return stack.node
	}
	return nil
}

func (t *tokens32) PrintSyntaxTree(buffer string) {
	t.AST().Print(buffer)
}

func (t *tokens32) PrettyPrintSyntaxTree(buffer string) {
	t.AST().PrettyPrint(buffer)
}

func (t *tokens32) Add(rule pegRule, begin, end, index uint32) {
	if tree := t.tree; int(index) >= len(tree) {
		expanded := make([]token32, 2*len(tree))
		copy(expanded, tree)
		t.tree = expanded
	}
	t.tree[index] = token32{
		pegRule: rule,
		begin:   begin,
		end:     end,
	}
}

func (t *tokens32) Tokens() []token32 {
	return t.tree
}

type StalogAST struct {
	Buffer string
	buffer []rune
	rules  [14]func() bool
	parse  func(rule ...int) error
	reset  func()
	Pretty bool
	tokens32
}

func (p *StalogAST) Parse(rule ...int) error {
	return p.parse(rule...)
}

func (p *StalogAST) Reset() {
	p.reset()
}

type textPosition struct {
	line, symbol int
}

type textPositionMap map[int]textPosition

func translatePositions(buffer []rune, positions []int) textPositionMap {
	length, translations, j, line, symbol := len(positions), make(textPositionMap, len(positions)), 0, 1, 0
	sort.Ints(positions)

search:
	for i, c := range buffer {
		if c == '\n' {
			line, symbol = line+1, 0
		} else {
			symbol++
		}
		if i == positions[j] {
			translations[positions[j]] = textPosition{line, symbol}
			for j++; j < length; j++ {
				if i != positions[j] {
					continue search
				}
			}
			break search
		}
	}

	return translations
}

type parseError struct {
	p   *StalogAST
	max token32
}

func (e *parseError) Error() string {
	tokens, error := []token32{e.max}, "\n"
	positions, p := make([]int, 2*len(tokens)), 0
	for _, token := range tokens {
		positions[p], p = int(token.begin), p+1
		positions[p], p = int(token.end), p+1
	}
	translations := translatePositions(e.p.buffer, positions)
	format := "parse error near %v (line %v symbol %v - line %v symbol %v):\n%v\n"
	if e.p.Pretty {
		format = "parse error near \x1B[34m%v\x1B[m (line %v symbol %v - line %v symbol %v):\n%v\n"
	}
	for _, token := range tokens {
		begin, end := int(token.begin), int(token.end)
		error += fmt.Sprintf(format,
			rul3s[token.pegRule],
			translations[begin].line, translations[begin].symbol,
			translations[end].line, translations[end].symbol,
			strconv.Quote(string(e.p.buffer[begin:end])))
	}

	return error
}

func (p *StalogAST) PrintSyntaxTree() {
	if p.Pretty {
		p.tokens32.PrettyPrintSyntaxTree(p.Buffer)
	} else {
		p.tokens32.PrintSyntaxTree(p.Buffer)
	}
}

func (p *StalogAST) Init() {
	var (
		max                  token32
		position, tokenIndex uint32
		buffer               []rune
	)
	p.reset = func() {
		max = token32{}
		position, tokenIndex = 0, 0

		p.buffer = []rune(p.Buffer)
		if len(p.buffer) == 0 || p.buffer[len(p.buffer)-1] != endSymbol {
			p.buffer = append(p.buffer, endSymbol)
		}
		buffer = p.buffer
	}
	p.reset()

	_rules := p.rules
	tree := tokens32{tree: make([]token32, math.MaxInt16)}
	p.parse = func(rule ...int) error {
		r := 1
		if len(rule) > 0 {
			r = rule[0]
		}
		matches := p.rules[r]()
		p.tokens32 = tree
		if matches {
			p.Trim(tokenIndex)
			return nil
		}
		return &parseError{p, max}
	}

	add := func(rule pegRule, begin uint32) {
		tree.Add(rule, begin, position, tokenIndex)
		tokenIndex++
		if begin != position && position > max.end {
			max = token32{rule, begin, position}
		}
	}

	matchDot := func() bool {
		if buffer[position] != endSymbol {
			position++
			return true
		}
		return false
	}

	/*matchChar := func(c byte) bool {
		if buffer[position] == c {
			position++
			return true
		}
		return false
	}*/

	/*matchRange := func(lower byte, upper byte) bool {
		if c := buffer[position]; c >= lower && c <= upper {
			position++
			return true
		}
		return false
	}*/

	_rules = [...]func() bool{
		nil,
		/* 0 Module <- <(Spacing ('p' 'a' 'c' 'k' 'a' 'g' 'e') Spacing Identifier Definition* EndOfFile)> */
		func() bool {
			position0, tokenIndex0 := position, tokenIndex
			{
				position1 := position
				if !_rules[ruleSpacing]() {
					goto l0
				}
				if buffer[position] != rune('p') {
					goto l0
				}
				position++
				if buffer[position] != rune('a') {
					goto l0
				}
				position++
				if buffer[position] != rune('c') {
					goto l0
				}
				position++
				if buffer[position] != rune('k') {
					goto l0
				}
				position++
				if buffer[position] != rune('a') {
					goto l0
				}
				position++
				if buffer[position] != rune('g') {
					goto l0
				}
				position++
				if buffer[position] != rune('e') {
					goto l0
				}
				position++
				if !_rules[ruleSpacing]() {
					goto l0
				}
				if !_rules[ruleIdentifier]() {
					goto l0
				}
			l2:
				{
					position3, tokenIndex3 := position, tokenIndex
					if !_rules[ruleDefinition]() {
						goto l3
					}
					goto l2
				l3:
					position, tokenIndex = position3, tokenIndex3
				}
				if !_rules[ruleEndOfFile]() {
					goto l0
				}
				add(ruleModule, position1)
			}
			return true
		l0:
			position, tokenIndex = position0, tokenIndex0
			return false
		},
		/* 1 Definition <- <SymbolDef> */
		func() bool {
			position4, tokenIndex4 := position, tokenIndex
			{
				position5 := position
				if !_rules[ruleSymbolDef]() {
					goto l4
				}
				add(ruleDefinition, position5)
			}
			return true
		l4:
			position, tokenIndex = position4, tokenIndex4
			return false
		},
		/* 2 SymbolDef <- <('s' 'y' 'm' 'b' 'o' 'l' Spacing SymbolName)> */
		func() bool {
			position6, tokenIndex6 := position, tokenIndex
			{
				position7 := position
				if buffer[position] != rune('s') {
					goto l6
				}
				position++
				if buffer[position] != rune('y') {
					goto l6
				}
				position++
				if buffer[position] != rune('m') {
					goto l6
				}
				position++
				if buffer[position] != rune('b') {
					goto l6
				}
				position++
				if buffer[position] != rune('o') {
					goto l6
				}
				position++
				if buffer[position] != rune('l') {
					goto l6
				}
				position++
				if !_rules[ruleSpacing]() {
					goto l6
				}
				if !_rules[ruleSymbolName]() {
					goto l6
				}
				add(ruleSymbolDef, position7)
			}
			return true
		l6:
			position, tokenIndex = position6, tokenIndex6
			return false
		},
		/* 3 Identifier <- <(SymbolName / DefName)> */
		func() bool {
			position8, tokenIndex8 := position, tokenIndex
			{
				position9 := position
				{
					position10, tokenIndex10 := position, tokenIndex
					if !_rules[ruleSymbolName]() {
						goto l11
					}
					goto l10
				l11:
					position, tokenIndex = position10, tokenIndex10
					if !_rules[ruleDefName]() {
						goto l8
					}
				}
			l10:
				add(ruleIdentifier, position9)
			}
			return true
		l8:
			position, tokenIndex = position8, tokenIndex8
			return false
		},
		/* 4 SymbolName <- <(<([A-Z] ([a-z] / [A-Z] / ([0-9] / [0-9]))*)> Spacing)> */
		func() bool {
			position12, tokenIndex12 := position, tokenIndex
			{
				position13 := position
				{
					position14 := position
					if c := buffer[position]; c < rune('A') || c > rune('Z') {
						goto l12
					}
					position++
				l15:
					{
						position16, tokenIndex16 := position, tokenIndex
						{
							position17, tokenIndex17 := position, tokenIndex
							if c := buffer[position]; c < rune('a') || c > rune('z') {
								goto l18
							}
							position++
							goto l17
						l18:
							position, tokenIndex = position17, tokenIndex17
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l19
							}
							position++
							goto l17
						l19:
							position, tokenIndex = position17, tokenIndex17
							{
								position20, tokenIndex20 := position, tokenIndex
								if c := buffer[position]; c < rune('0') || c > rune('9') {
									goto l21
								}
								position++
								goto l20
							l21:
								position, tokenIndex = position20, tokenIndex20
								if c := buffer[position]; c < rune('0') || c > rune('9') {
									goto l16
								}
								position++
							}
						l20:
						}
					l17:
						goto l15
					l16:
						position, tokenIndex = position16, tokenIndex16
					}
					add(rulePegText, position14)
				}
				if !_rules[ruleSpacing]() {
					goto l12
				}
				add(ruleSymbolName, position13)
			}
			return true
		l12:
			position, tokenIndex = position12, tokenIndex12
			return false
		},
		/* 5 DefName <- <(<([a-z] ([a-z] / [A-Z] / ([0-9] / [0-9]))*)> Spacing)> */
		func() bool {
			position22, tokenIndex22 := position, tokenIndex
			{
				position23 := position
				{
					position24 := position
					if c := buffer[position]; c < rune('a') || c > rune('z') {
						goto l22
					}
					position++
				l25:
					{
						position26, tokenIndex26 := position, tokenIndex
						{
							position27, tokenIndex27 := position, tokenIndex
							if c := buffer[position]; c < rune('a') || c > rune('z') {
								goto l28
							}
							position++
							goto l27
						l28:
							position, tokenIndex = position27, tokenIndex27
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l29
							}
							position++
							goto l27
						l29:
							position, tokenIndex = position27, tokenIndex27
							{
								position30, tokenIndex30 := position, tokenIndex
								if c := buffer[position]; c < rune('0') || c > rune('9') {
									goto l31
								}
								position++
								goto l30
							l31:
								position, tokenIndex = position30, tokenIndex30
								if c := buffer[position]; c < rune('0') || c > rune('9') {
									goto l26
								}
								position++
							}
						l30:
						}
					l27:
						goto l25
					l26:
						position, tokenIndex = position26, tokenIndex26
					}
					add(rulePegText, position24)
				}
				if !_rules[ruleSpacing]() {
					goto l22
				}
				add(ruleDefName, position23)
			}
			return true
		l22:
			position, tokenIndex = position22, tokenIndex22
			return false
		},
		/* 6 Space <- <(WhiteSpace / Comment)> */
		func() bool {
			position32, tokenIndex32 := position, tokenIndex
			{
				position33 := position
				{
					position34, tokenIndex34 := position, tokenIndex
					if !_rules[ruleWhiteSpace]() {
						goto l35
					}
					goto l34
				l35:
					position, tokenIndex = position34, tokenIndex34
					if !_rules[ruleComment]() {
						goto l32
					}
				}
			l34:
				add(ruleSpace, position33)
			}
			return true
		l32:
			position, tokenIndex = position32, tokenIndex32
			return false
		},
		/* 7 Spacing <- <Space*> */
		func() bool {
			{
				position37 := position
			l38:
				{
					position39, tokenIndex39 := position, tokenIndex
					if !_rules[ruleSpace]() {
						goto l39
					}
					goto l38
				l39:
					position, tokenIndex = position39, tokenIndex39
				}
				add(ruleSpacing, position37)
			}
			return true
		},
		/* 8 WhiteSpace <- <(' ' / '\n' / '\r' / '\t')> */
		func() bool {
			position40, tokenIndex40 := position, tokenIndex
			{
				position41 := position
				{
					position42, tokenIndex42 := position, tokenIndex
					if buffer[position] != rune(' ') {
						goto l43
					}
					position++
					goto l42
				l43:
					position, tokenIndex = position42, tokenIndex42
					if buffer[position] != rune('\n') {
						goto l44
					}
					position++
					goto l42
				l44:
					position, tokenIndex = position42, tokenIndex42
					if buffer[position] != rune('\r') {
						goto l45
					}
					position++
					goto l42
				l45:
					position, tokenIndex = position42, tokenIndex42
					if buffer[position] != rune('\t') {
						goto l40
					}
					position++
				}
			l42:
				add(ruleWhiteSpace, position41)
			}
			return true
		l40:
			position, tokenIndex = position40, tokenIndex40
			return false
		},
		/* 9 Comment <- <('#' (!EndOfLine .)* EndOfLine)> */
		func() bool {
			position46, tokenIndex46 := position, tokenIndex
			{
				position47 := position
				if buffer[position] != rune('#') {
					goto l46
				}
				position++
			l48:
				{
					position49, tokenIndex49 := position, tokenIndex
					{
						position50, tokenIndex50 := position, tokenIndex
						if !_rules[ruleEndOfLine]() {
							goto l50
						}
						goto l49
					l50:
						position, tokenIndex = position50, tokenIndex50
					}
					if !matchDot() {
						goto l49
					}
					goto l48
				l49:
					position, tokenIndex = position49, tokenIndex49
				}
				if !_rules[ruleEndOfLine]() {
					goto l46
				}
				add(ruleComment, position47)
			}
			return true
		l46:
			position, tokenIndex = position46, tokenIndex46
			return false
		},
		/* 10 EndOfFile <- <!.> */
		func() bool {
			position51, tokenIndex51 := position, tokenIndex
			{
				position52 := position
				{
					position53, tokenIndex53 := position, tokenIndex
					if !matchDot() {
						goto l53
					}
					goto l51
				l53:
					position, tokenIndex = position53, tokenIndex53
				}
				add(ruleEndOfFile, position52)
			}
			return true
		l51:
			position, tokenIndex = position51, tokenIndex51
			return false
		},
		/* 11 EndOfLine <- <'\n'> */
		func() bool {
			position54, tokenIndex54 := position, tokenIndex
			{
				position55 := position
				if buffer[position] != rune('\n') {
					goto l54
				}
				position++
				add(ruleEndOfLine, position55)
			}
			return true
		l54:
			position, tokenIndex = position54, tokenIndex54
			return false
		},
		nil,
	}
	p.rules = _rules
}
