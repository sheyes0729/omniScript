package token

import "fmt"

type TokenType string

const (
	ILLEGAL = "ILLEGAL"
	EOF     = "EOF"

	// 标识符 + 字面量
	IDENT  = "IDENT"  // add, foobar, x, y, ...
	INT    = "INT"    // 1343456
	STRING = "STRING" // "foobar"

	// 运算符
	ASSIGN   = "="
	PLUS     = "+"
	MINUS    = "-"
	BANG     = "!"
	ASTERISK = "*"
	SLASH    = "/"

	LT = "<"
	GT = ">"

	EQ     = "=="
	NOT_EQ = "!="

	// 分隔符
	COMMA     = ","
	SEMICOLON = ";"
	COLON     = ":"
	DOT       = "."

	LPAREN   = "("
	RPAREN   = ")"
	LBRACE   = "{"
	RBRACE   = "}"
	LBRACKET = "["
	RBRACKET = "]"

	// 关键字
	FUNCTION = "FUNCTION"
	LET      = "LET"
	CONST    = "CONST"
	TRUE     = "TRUE"
	FALSE    = "FALSE"
	IF       = "IF"
	ELSE     = "ELSE"
	RETURN   = "RETURN"
	WHILE    = "WHILE"
	DECLARE  = "DECLARE"
	CLASS    = "CLASS"
	NEW      = "NEW"
	THIS     = "THIS"
	EXTENDS  = "EXTENDS"
	SUPER    = "SUPER"
	INTERFACE  = "INTERFACE"
	IMPLEMENTS = "IMPLEMENTS"
	SPAWN      = "SPAWN"
	ENUM       = "ENUM"
	TYPE       = "TYPE"
	FOR        = "FOR"
)

type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Column  int
}

func (t Token) String() string {
	return fmt.Sprintf("Token(%s, '%s', Line:%d)", t.Type, t.Literal, t.Line)
}

var keywords = map[string]TokenType{
	"function": FUNCTION,
	"let":      LET,
	"const":    CONST,
	"true":     TRUE,
	"false":    FALSE,
	"if":       IF,
	"else":     ELSE,
	"return":   RETURN,
	"while":    WHILE,
	"declare":  DECLARE,
	"class":    CLASS,
	"new":      NEW,
	"this":     THIS,
	"extends":  EXTENDS,
	"super":    SUPER,
	"interface": INTERFACE,
	"implements": IMPLEMENTS,
	"spawn":      SPAWN,
	"enum":       ENUM,
	"type":       TYPE,
	"for":        FOR,
}

func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}
