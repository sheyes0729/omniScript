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

// EnumMember represents a member of an enum
type EnumMember struct {
	Name  *Identifier
	Value Expression // Can be nil (auto-increment)
}

func (em *EnumMember) String() string {
	var out bytes.Buffer
	out.WriteString(em.Name.String())
	if em.Value != nil {
		out.WriteString(" = ")
		out.WriteString(em.Value.String())
	}
	return out.String()
}

// EnumStatement represents an enum definition
type EnumStatement struct {
	Token   token.Token // token.ENUM
	Name    *Identifier
	Members []*EnumMember
}

func (es *EnumStatement) statementNode()       {}
func (es *EnumStatement) TokenLiteral() string { return es.Token.Literal }
func (es *EnumStatement) String() string {
	var out bytes.Buffer
	out.WriteString("enum ")
	out.WriteString(es.Name.String())
	out.WriteString(" { ")
	for i, m := range es.Members {
		out.WriteString(m.String())
		if i < len(es.Members)-1 {
			out.WriteString(", ")
		}
	}
	out.WriteString(" }")
	return out.String()
}

// InterfaceStatement definition
type InterfaceStatement struct {
	Token   token.Token // token.INTERFACE
	Name    *Identifier
	Methods []*MethodSignature
}

func (is *InterfaceStatement) statementNode()       {}
func (is *InterfaceStatement) TokenLiteral() string { return is.Token.Literal }
func (is *InterfaceStatement) String() string {
	var out bytes.Buffer
	out.WriteString("interface ")
	out.WriteString(is.Name.String())
	out.WriteString(" { ")
	for _, m := range is.Methods {
		out.WriteString(m.String())
		out.WriteString("; ")
	}
	out.WriteString(" }")
	return out.String()
}

// TypeAliasStatement represents type Name = Type;
type TypeAliasStatement struct {
	Token token.Token // token.TYPE
	Name  *Identifier
	Value string // The type name
}

func (tas *TypeAliasStatement) statementNode()       {}
func (tas *TypeAliasStatement) TokenLiteral() string { return tas.Token.Literal }
func (tas *TypeAliasStatement) String() string {
	var out bytes.Buffer
	out.WriteString("type ")
	out.WriteString(tas.Name.String())
	out.WriteString(" = ")
	out.WriteString(tas.Value)
	out.WriteString(";")
	return out.String()
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
	Token token.Token // token.INT
	Value int64
}

func (il *IntegerLiteral) expressionNode()      {}
func (il *IntegerLiteral) TokenLiteral() string { return il.Token.Literal }
func (il *IntegerLiteral) String() string       { return il.Token.Literal }

// StringLiteral 字符串
type StringLiteral struct {
	Token token.Token // token.STRING
	Value string
}

func (sl *StringLiteral) expressionNode()      {}
func (sl *StringLiteral) TokenLiteral() string { return sl.Token.Literal }
func (sl *StringLiteral) String() string       { return sl.Token.Literal }

// PrefixExpression 前缀表达式
type PrefixExpression struct {
	Token    token.Token // 前缀操作符 token, e.g. !
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

// InfixExpression 中缀表达式
type InfixExpression struct {
	Token    token.Token // 中缀操作符 token, e.g. +
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

// LetStatement let 语句
type LetStatement struct {
	Token token.Token // token.LET
	Name  *Identifier
	Type  string // Optional type annotation
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

// ReturnStatement return 语句
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

// ExpressionStatement 表达式语句
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

// BlockStatement 块语句
type BlockStatement struct {
	Token      token.Token // {
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

// IfExpression if 表达式
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

// FunctionLiteral 函数字面量
type FunctionLiteral struct {
	Token      token.Token // 'fn'
	Parameters []*FieldDefinition // Parameters are fields (name: type)
	Body       *BlockStatement
	Name       string // Optional name
	ReturnType string // Optional return type
}

func (fl *FunctionLiteral) expressionNode()      {}
func (fl *FunctionLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FunctionLiteral) String() string {
	var out bytes.Buffer
	out.WriteString(fl.TokenLiteral())
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

// CallExpression 调用表达式
type CallExpression struct {
	Token     token.Token // '('
	Function  Expression  // Identifier or FunctionLiteral
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

// ArrayLiteral 数组字面量
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

// MapLiteral Map字面量
type MapLiteral struct {
	Token token.Token // '{'
	Pairs map[Expression]Expression
}

func (ml *MapLiteral) expressionNode()      {}
func (ml *MapLiteral) TokenLiteral() string { return ml.Token.Literal }
func (ml *MapLiteral) String() string {
	var out bytes.Buffer
	out.WriteString("{")
	for key, value := range ml.Pairs {
		out.WriteString(key.String())
		out.WriteString(":")
		out.WriteString(value.String())
		out.WriteString(", ")
	}
	out.WriteString("}")
	return out.String()
}

// IndexExpression 索引表达式
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

// WhileStatement while 循环
type WhileStatement struct {
	Token     token.Token // token.WHILE
	Condition Expression
	Body      *BlockStatement
}

func (ws *WhileStatement) statementNode()       {}
func (ws *WhileStatement) TokenLiteral() string { return ws.Token.Literal }
func (ws *WhileStatement) String() string {
	var out bytes.Buffer
	out.WriteString("while")
	out.WriteString(ws.Condition.String())
	out.WriteString(" ")
	out.WriteString(ws.Body.String())
	return out.String()
}

// ForStatement for loop
type ForStatement struct {
	Token     token.Token // token.FOR
	Init      Statement
	Condition Expression
	Update    Statement
	Body      *BlockStatement
}

func (fs *ForStatement) statementNode()       {}
func (fs *ForStatement) TokenLiteral() string { return fs.Token.Literal }
func (fs *ForStatement) String() string {
	var out bytes.Buffer
	out.WriteString("for (")
	if fs.Init != nil {
		out.WriteString(fs.Init.String())
	}
	out.WriteString("; ")
	if fs.Condition != nil {
		out.WriteString(fs.Condition.String())
	}
	out.WriteString("; ")
	if fs.Update != nil {
		out.WriteString(fs.Update.String())
	}
	out.WriteString(") ")
	out.WriteString(fs.Body.String())
	return out.String()
}

// FieldDefinition represents a field/parameter with type
type FieldDefinition struct {
	Token token.Token
	Name  *Identifier
	Type  string // "int", "string", "void", "bool"
	Value Expression
}

func (fd *FieldDefinition) String() string {
	return fd.Name.String() + ": " + fd.Type
}

// ImportStatement represents a declare statement
type ImportStatement struct {
	Token      token.Token // token.DECLARE
	Name       *Identifier
	Parameters []*FieldDefinition
	ReturnType string
}

func (is *ImportStatement) statementNode()       {}
func (is *ImportStatement) TokenLiteral() string { return is.Token.Literal }
func (is *ImportStatement) String() string {
	return "declare function " + is.Name.String()
}

// ClassStatement represents a class definition
type ClassStatement struct {
	Token   token.Token // token.CLASS
	Name    *Identifier
	Fields  []*FieldDefinition
	Methods []*FunctionLiteral
	Parent  *Identifier // Optional parent class
	Implements []*Identifier // Implemented interfaces
}

func (cs *ClassStatement) statementNode()       {}
func (cs *ClassStatement) TokenLiteral() string { return cs.Token.Literal }
func (cs *ClassStatement) String() string {
	return "class " + cs.Name.String()
}

// NewExpression represents new Class()
type NewExpression struct {
	Token     token.Token // token.NEW
	Class     *Identifier
	Arguments []Expression
}

func (ne *NewExpression) expressionNode()      {}
func (ne *NewExpression) TokenLiteral() string { return ne.Token.Literal }
func (ne *NewExpression) String() string {
	return "new " + ne.Class.String()
}

// ThisExpression represents 'this'
type ThisExpression struct {
	Token token.Token // token.THIS
}

func (te *ThisExpression) expressionNode()      {}
func (te *ThisExpression) TokenLiteral() string { return te.Token.Literal }
func (te *ThisExpression) String() string {
	return "this"
}

// MemberExpression represents obj.prop
type MemberExpression struct {
	Token    token.Token // token.DOT
	Object   Expression
	Property *Identifier
}

func (me *MemberExpression) expressionNode()      {}
func (me *MemberExpression) TokenLiteral() string { return me.Token.Literal }
func (me *MemberExpression) String() string {
	return me.Object.String() + "." + me.Property.String()
}

// AssignmentExpression represents left = right
type AssignmentExpression struct {
	Token token.Token // token.ASSIGN
	Left  Expression
	Value Expression
}

func (ae *AssignmentExpression) expressionNode()      {}
func (ae *AssignmentExpression) TokenLiteral() string { return ae.Token.Literal }
func (ae *AssignmentExpression) String() string {
	return ae.Left.String() + " = " + ae.Value.String()
}

// SuperExpression represents 'super'
type SuperExpression struct {
	Token token.Token // token.SUPER
}

func (se *SuperExpression) expressionNode()      {}
func (se *SuperExpression) TokenLiteral() string { return se.Token.Literal }
func (se *SuperExpression) String() string {
	return "super"
}

// MethodSignature represents method in interface
type MethodSignature struct {
	Token      token.Token
	Name       string
	Parameters []*FieldDefinition
	ReturnType string
}

func (ms *MethodSignature) String() string {
	return ms.Name
}

// SpawnStatement represents spawn func()
type SpawnStatement struct {
	Token token.Token // token.SPAWN
	Call  *CallExpression
}

func (ss *SpawnStatement) statementNode()       {}
func (ss *SpawnStatement) TokenLiteral() string { return ss.Token.Literal }
func (ss *SpawnStatement) String() string {
	return "spawn " + ss.Call.String()
}

// ImportModuleStatement represents import { x, y } from "module";
type ImportModuleStatement struct {
	Token       token.Token // token.IMPORT
	Identifiers []*Identifier
	Source      string
}

func (ims *ImportModuleStatement) statementNode()       {}
func (ims *ImportModuleStatement) TokenLiteral() string { return ims.Token.Literal }
func (ims *ImportModuleStatement) String() string {
	var out bytes.Buffer
	out.WriteString("import { ")
	for i, ident := range ims.Identifiers {
		out.WriteString(ident.String())
		if i < len(ims.Identifiers)-1 {
			out.WriteString(", ")
		}
	}
	out.WriteString(" } from \"")
	out.WriteString(ims.Source)
	out.WriteString("\";")
	return out.String()
}

// ExportStatement represents export function/class/var ...
type ExportStatement struct {
	Token     token.Token // token.EXPORT
	Statement Statement
}

func (es *ExportStatement) statementNode()       {}
func (es *ExportStatement) TokenLiteral() string { return es.Token.Literal }
func (es *ExportStatement) String() string {
	return "export " + es.Statement.String()
}
