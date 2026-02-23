package ast

import (
	"bytes"
	"omniScript/pkg/token"
)

// Node 是 AST 中的基本节点接口
type Node interface {
	TokenLiteral() string
	String() string
}

// Statement 是语句节点（不产生值的代码，如 let x = 5; return;）
type Statement interface {
	Node
	statementNode()
}

// Expression 是表达式节点（产生值的代码，如 5 + 5, add(1, 2)）
type Expression interface {
	Node
	expressionNode()
}

// Program 是 AST 的根节点
type Program struct {
	Statements []Statement
}

func (p *Program) TokenLiteral() string {
	if len(p.Statements) > 0 {
		return p.Statements[0].TokenLiteral()
	}
	return ""
}

func (p *Program) String() string {
	var out bytes.Buffer
	for _, s := range p.Statements {
		out.WriteString(s.String())
	}
	return out.String()
}

// LetStatement 表示 let 语句
type LetStatement struct {
	Token token.Token // token.LET
	Name  *Identifier
	Value Expression
}

func (ls *LetStatement) statementNode()       {}
func (ls *LetStatement) TokenLiteral() string { return ls.Token.Literal }
func (ls *LetStatement) String() string {
	var out bytes.Buffer
	out.WriteString(ls.TokenLiteral() + " ")
	out.WriteString(ls.Name.String())
	out.WriteString(" = ")
	if ls.Value != nil {
		out.WriteString(ls.Value.String())
	}
	out.WriteString(";")
	return out.String()
}

// ReturnStatement 表示 return 语句
type ReturnStatement struct {
	Token       token.Token // token.RETURN
	ReturnValue Expression
}

func (rs *ReturnStatement) statementNode()       {}
func (rs *ReturnStatement) TokenLiteral() string { return rs.Token.Literal }
func (rs *ReturnStatement) String() string {
	var out bytes.Buffer
	out.WriteString(rs.TokenLiteral() + " ")
	if rs.ReturnValue != nil {
		out.WriteString(rs.ReturnValue.String())
	}
	out.WriteString(";")
	return out.String()
}

// WhileStatement 循环
type WhileStatement struct {
	Token     token.Token // 'while'
	Condition Expression
	Body      *BlockStatement
}

func (ws *WhileStatement) statementNode()       {}
func (ws *WhileStatement) TokenLiteral() string { return ws.Token.Literal }
func (ws *WhileStatement) String() string {
	var out bytes.Buffer
	out.WriteString("while (")
	if ws.Condition != nil {
		out.WriteString(ws.Condition.String())
	}
	out.WriteString(") ")
	out.WriteString(ws.Body.String())
	return out.String()
}

// ImportStatement 外部函数声明
type ImportStatement struct {
	Token      token.Token // 'declare'
	Name       *Identifier
	Parameters []*FieldDefinition // Reuse FieldDefinition for parameters (Name: Type)
	ReturnType string             // "void", "int", "string"
}

func (is *ImportStatement) statementNode()       {}
func (is *ImportStatement) TokenLiteral() string { return is.Token.Literal }
func (is *ImportStatement) String() string {
	var out bytes.Buffer
	out.WriteString("declare function ")
	out.WriteString(is.Name.String())
	out.WriteString("(")
	for i, p := range is.Parameters {
		out.WriteString(p.Name.String())
		if p.Type != "" {
			out.WriteString(": " + p.Type)
		}
		if i < len(is.Parameters)-1 {
			out.WriteString(", ")
		}
	}
	out.WriteString(")")
	if is.ReturnType != "" {
		out.WriteString(": " + is.ReturnType)
	}
	out.WriteString(";")
	return out.String()
}

// ExpressionStatement 是为了让表达式可以作为语句出现（例如单独一行的函数调用）
type ExpressionStatement struct {
	Token      token.Token // 表达式的第一个 token
	Expression Expression
}

func (es *ExpressionStatement) statementNode()       {}
func (es *ExpressionStatement) TokenLiteral() string { return es.Token.Literal }
func (es *ExpressionStatement) String() string {
	if es.Expression != nil {
		return es.Expression.String()
	}
	return ""
}

// Identifier 标识符
type Identifier struct {
	Token token.Token // token.IDENT
	Value string
}

func (i *Identifier) expressionNode()      {}
func (i *Identifier) TokenLiteral() string { return i.Token.Literal }
func (i *Identifier) String() string       { return i.Value }

// IntegerLiteral 整数
type IntegerLiteral struct {
	Token token.Token
	Value int64
}

func (il *IntegerLiteral) expressionNode()      {}
func (il *IntegerLiteral) TokenLiteral() string { return il.Token.Literal }
func (il *IntegerLiteral) String() string       { return il.Token.Literal }

// PrefixExpression 前缀表达式 (!true, -5)
type PrefixExpression struct {
	Token    token.Token // 前缀 token, 如 ! 或 -
	Operator string
	Right    Expression
}

func (pe *PrefixExpression) expressionNode()      {}
func (pe *PrefixExpression) TokenLiteral() string { return pe.Token.Literal }
func (pe *PrefixExpression) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(pe.Operator)
	out.WriteString(pe.Right.String())
	out.WriteString(")")
	return out.String()
}

// InfixExpression 中缀表达式 (5 + 5, a == b)
type InfixExpression struct {
	Token    token.Token // 运算符 token, 如 +
	Left     Expression
	Operator string
	Right    Expression
}

func (ie *InfixExpression) expressionNode()      {}
func (ie *InfixExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *InfixExpression) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(ie.Left.String())
	out.WriteString(" " + ie.Operator + " ")
	out.WriteString(ie.Right.String())
	out.WriteString(")")
	return out.String()
}

// Boolean 布尔值
type Boolean struct {
	Token token.Token
	Value bool
}

func (b *Boolean) expressionNode()      {}
func (b *Boolean) TokenLiteral() string { return b.Token.Literal }
func (b *Boolean) String() string       { return b.Token.Literal }

// IfExpression If 表达式
type IfExpression struct {
	Token       token.Token // 'if'
	Condition   Expression
	Consequence *BlockStatement
	Alternative *BlockStatement
}

func (ie *IfExpression) expressionNode()      {}
func (ie *IfExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *IfExpression) String() string {
	var out bytes.Buffer
	out.WriteString("if")
	out.WriteString(ie.Condition.String())
	out.WriteString(" ")
	out.WriteString(ie.Consequence.String())
	if ie.Alternative != nil {
		out.WriteString("else ")
		out.WriteString(ie.Alternative.String())
	}
	return out.String()
}

// BlockStatement 块语句 { ... }
type BlockStatement struct {
	Token      token.Token // '{'
	Statements []Statement
}

func (bs *BlockStatement) statementNode()       {}
func (bs *BlockStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BlockStatement) String() string {
	var out bytes.Buffer
	for _, s := range bs.Statements {
		out.WriteString(s.String())
	}
	return out.String()
}

// FunctionLiteral 函数定义 function(x, y) { ... }
type FunctionLiteral struct {
	Token      token.Token // 'function'
	Parameters []*Identifier
	Body       *BlockStatement
	Name       string // 可选，对于匿名函数为空
}

func (fl *FunctionLiteral) expressionNode()      {}
func (fl *FunctionLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FunctionLiteral) String() string {
	var out bytes.Buffer
	out.WriteString(fl.TokenLiteral())
	if fl.Name != "" {
		out.WriteString(" " + fl.Name)
	}
	out.WriteString("(")
	for i, p := range fl.Parameters {
		out.WriteString(p.String())
		if i < len(fl.Parameters)-1 {
			out.WriteString(", ")
		}
	}
	out.WriteString(") ")
	out.WriteString(fl.Body.String())
	return out.String()
}

// CallExpression 函数调用 add(1, 2)
type CallExpression struct {
	Token     token.Token // '('
	Function  Expression  // 标识符或函数字面量
	Arguments []Expression
}

func (ce *CallExpression) expressionNode()      {}
func (ce *CallExpression) TokenLiteral() string { return ce.Token.Literal }
func (ce *CallExpression) String() string {
	var out bytes.Buffer
	out.WriteString(ce.Function.String())
	out.WriteString("(")
	for i, a := range ce.Arguments {
		out.WriteString(a.String())
		if i < len(ce.Arguments)-1 {
			out.WriteString(", ")
		}
	}
	out.WriteString(")")
	return out.String()
}

// StringLiteral 字符串
type StringLiteral struct {
	Token token.Token
	Value string
}

func (sl *StringLiteral) expressionNode()      {}
func (sl *StringLiteral) TokenLiteral() string { return sl.Token.Literal }
func (sl *StringLiteral) String() string       { return sl.Token.Literal }

// ArrayLiteral 数组字面量 [1, 2, 3]
type ArrayLiteral struct {
	Token    token.Token // '['
	Elements []Expression
}

func (al *ArrayLiteral) expressionNode()      {}
func (al *ArrayLiteral) TokenLiteral() string { return al.Token.Literal }
func (al *ArrayLiteral) String() string {
	var out bytes.Buffer
	out.WriteString("[")
	for i, el := range al.Elements {
		out.WriteString(el.String())
		if i < len(al.Elements)-1 {
			out.WriteString(", ")
		}
	}
	out.WriteString("]")
	return out.String()
}

// MapLiteral { key: val, ... }
type MapLiteral struct {
	Token token.Token // '{'
	Pairs map[Expression]Expression // Only supports String keys effectively for now
}

func (ml *MapLiteral) expressionNode() {}
func (ml *MapLiteral) TokenLiteral() string { return ml.Token.Literal }
func (ml *MapLiteral) String() string {
	var out bytes.Buffer
	out.WriteString("{")
	i := 0
	for key, value := range ml.Pairs {
		if i > 0 {
			out.WriteString(", ")
		}
		out.WriteString(key.String())
		out.WriteString(":")
		out.WriteString(value.String())
		i++
	}
	out.WriteString("}")
	return out.String()
}

// IndexExpression 数组/Map索引 arr[1], map["key"]
type IndexExpression struct {
	Token token.Token // '['
	Left  Expression
	Index Expression
}

func (ie *IndexExpression) expressionNode()      {}
func (ie *IndexExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *IndexExpression) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(ie.Left.String())
	out.WriteString("[")
	out.WriteString(ie.Index.String())
	out.WriteString("])")
	return out.String()
}

// ClassStatement 类定义 class Person extends Animal { ... }
type ClassStatement struct {
	Token  token.Token // 'class'
	Name   *Identifier
	Parent *Identifier // 可选的父类
	Fields []*FieldDefinition
	Methods []*FunctionLiteral
}

func (cs *ClassStatement) statementNode()       {}
func (cs *ClassStatement) TokenLiteral() string { return cs.Token.Literal }
func (cs *ClassStatement) String() string {
	var out bytes.Buffer
	out.WriteString("class ")
	out.WriteString(cs.Name.String())
	if cs.Parent != nil {
		out.WriteString(" extends " + cs.Parent.String())
	}
	out.WriteString(" { ")
	for _, f := range cs.Fields {
		out.WriteString(f.String())
	}
	for _, m := range cs.Methods {
		out.WriteString(m.String())
	}
	out.WriteString(" }")
	return out.String()
}

// FieldDefinition 类字段定义 name: type = value;
type FieldDefinition struct {
	Token token.Token // 字段名 token
	Name  *Identifier
	Type  string     // 类型标注 (可选)
	Value Expression // 初始值 (可选)
}

func (fd *FieldDefinition) statementNode()       {}
func (fd *FieldDefinition) TokenLiteral() string { return fd.Token.Literal }
func (fd *FieldDefinition) String() string {
	var out bytes.Buffer
	out.WriteString(fd.Name.String())
	if fd.Type != "" {
		out.WriteString(": " + fd.Type)
	}
	if fd.Value != nil {
		out.WriteString(" = ")
		out.WriteString(fd.Value.String())
	}
	out.WriteString(";")
	return out.String()
}

// NewExpression 对象创建 new Person()
type NewExpression struct {
	Token     token.Token // 'new'
	Class     *Identifier
	Arguments []Expression
}

func (ne *NewExpression) expressionNode()      {}
func (ne *NewExpression) TokenLiteral() string { return ne.Token.Literal }
func (ne *NewExpression) String() string {
	var out bytes.Buffer
	out.WriteString("new ")
	out.WriteString(ne.Class.String())
	out.WriteString("(")
	for i, arg := range ne.Arguments {
		out.WriteString(arg.String())
		if i < len(ne.Arguments)-1 {
			out.WriteString(", ")
		}
	}
	out.WriteString(")")
	return out.String()
}

// MemberExpression 成员访问 obj.prop
type MemberExpression struct {
	Token    token.Token // '.'
	Object   Expression
	Property *Identifier
}

func (me *MemberExpression) expressionNode()      {}
func (me *MemberExpression) TokenLiteral() string { return me.Token.Literal }
func (me *MemberExpression) String() string {
	var out bytes.Buffer
	out.WriteString(me.Object.String())
	out.WriteString(".")
	out.WriteString(me.Property.String())
	return out.String()
}

// ThisExpression 'this' 关键字
type ThisExpression struct {
	Token token.Token // 'this'
}

func (te *ThisExpression) expressionNode()      {}
func (te *ThisExpression) TokenLiteral() string { return te.Token.Literal }
func (te *ThisExpression) String() string       { return "this" }

// SuperExpression 'super' 关键字 (super.method())
type SuperExpression struct {
	Token token.Token // 'super'
}

func (se *SuperExpression) expressionNode()      {}
func (se *SuperExpression) TokenLiteral() string { return se.Token.Literal }
func (se *SuperExpression) String() string       { return "super" }

// AssignmentExpression 赋值 x = 5, obj.prop = 5
type AssignmentExpression struct {
	Token token.Token // '='
	Left  Expression
	Value Expression
}

func (ae *AssignmentExpression) expressionNode()      {}
func (ae *AssignmentExpression) TokenLiteral() string { return ae.Token.Literal }
func (ae *AssignmentExpression) String() string {
	var out bytes.Buffer
	out.WriteString(ae.Left.String())
	out.WriteString(" = ")
	out.WriteString(ae.Value.String())
	return out.String()
}
