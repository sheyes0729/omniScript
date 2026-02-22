package compiler

import (
	"bytes"
	"fmt"
	"omniScript/pkg/ast"
)

type DataType string

const (
	TypeInt     DataType = "int"
	TypeString  DataType = "string"
	TypeVoid    DataType = "void"
	TypeBool    DataType = "bool"
	TypeArray   DataType = "array"
	TypeUnknown DataType = "unknown"
)

const stdLibWAT = `
;; --- Built-in Memory & String Library ---
(global $heap_ptr (mut i32) (i32.const 10240)) 
(global $free_list (mut i32) (i32.const 0))
(global $shadow_stack_base (mut i32) (i32.const 1024))
(global $shadow_stack_ptr (mut i32) (i32.const 1024))
(global $allocated_list (mut i32) (i32.const 0))

(func $malloc (param $size i32) (param $type_id i32) (result i32)
  (local $curr i32)
  (local $prev i32)
  (local $block_size i32)
  
  ;; Add 16 bytes for header: [next_allocated (4)] [size (4)] [mark (4)] [type_id (4)]
  local.get $size
  i32.const 16
  i32.add
  local.set $size

  global.get $free_list
  local.set $curr
  i32.const 0
  local.set $prev
  
  (block $found
    (loop $search
      local.get $curr
      i32.eqz
      br_if $found
      
      local.get $curr
      i32.const 4
      i32.add
      i32.load ;; Load size from offset 4
      local.set $block_size
      
      local.get $block_size
      local.get $size
      i32.ge_u
      if
        ;; Found a block
        local.get $prev
        i32.eqz
        if
          local.get $curr
          i32.const 8 ;; next free is at offset 8 (was 4)
          i32.add
          i32.load
          global.set $free_list
        else
          local.get $prev
          i32.const 8
          i32.add
          local.get $curr
          i32.const 8
          i32.add
          i32.load
          i32.store
        end
        
        ;; Add to allocated list
        local.get $curr
        global.get $allocated_list
        i32.store ;; store next_allocated at offset 0
        local.get $curr
        global.set $allocated_list
        
        ;; Initialize mark bit (offset 8) to 0
        local.get $curr
        i32.const 8
        i32.add
        i32.const 0
        i32.store

        ;; Initialize type_id (offset 12)
        local.get $curr
        i32.const 12
        i32.add
        local.get $type_id
        i32.store
        
        ;; Return payload pointer (curr + 16)
        local.get $curr
        i32.const 16
        i32.add
        return
      end
      
      local.get $curr
      local.set $prev
      local.get $curr
      i32.const 8 ;; next free is at offset 8
      i32.add
      i32.load
      local.set $curr
      br $search
    )
  )
  
  ;; No free block found, allocate new memory
  local.get $size
  i32.const 8
  i32.add
  local.set $block_size
  
  global.get $heap_ptr
  local.set $curr
  
  ;; Set size at offset 4
  local.get $curr
  i32.const 4
  i32.add
  local.get $size
  i32.store
  
  ;; Add to allocated list
  local.get $curr
  global.get $allocated_list
  i32.store ;; store next_allocated at offset 0
  local.get $curr
  global.set $allocated_list

  ;; Initialize mark bit (offset 8) to 0
  local.get $curr
  i32.const 8
  i32.add
  i32.const 0
  i32.store

  ;; Initialize type_id (offset 12)
  local.get $curr
  i32.const 12
  i32.add
  local.get $type_id
  i32.store

  ;; Update heap pointer
  global.get $heap_ptr
  local.get $size ;; size includes header
  i32.add
  global.set $heap_ptr
  
  ;; Return payload pointer
  local.get $curr
  i32.const 16
  i32.add
)

(func $gc_mark (param $ptr i32)
  (local $header i32)
  (local $type_id i32)
  
  local.get $ptr
  i32.eqz
  if
    return
  end
  
  ;; Get header (ptr - 16)
  local.get $ptr
  i32.const 16
  i32.sub
  local.set $header
  
  ;; Check if already marked (offset 8)
  local.get $header
  i32.const 8
  i32.add
  i32.load
  if
    return
  end
  
  ;; Mark it
  local.get $header
  i32.const 8
  i32.add
  i32.const 1
  i32.store
  
  ;; Trace children
  ;; Get type_id (offset 12)
  local.get $header
  i32.const 12
  i32.add
  i32.load
  local.set $type_id
  
  local.get $ptr
  local.get $type_id
  call $gc_trace
)

(func $gc_collect
  (local $scan_ptr i32)
  (local $curr i32)
  (local $prev i32)
  (local $next i32)
  (local $marked i32)
  
  ;; 1. Mark Phase
  ;; Scan Shadow Stack
  global.get $shadow_stack_base
  local.set $scan_ptr
  
  (block $done_scan
    (loop $scan
       local.get $scan_ptr
       global.get $shadow_stack_ptr
       i32.ge_u
       br_if $done_scan
       
       local.get $scan_ptr
       i32.load
       call $gc_mark
       
       local.get $scan_ptr
       i32.const 4
       i32.add
       local.set $scan_ptr
       br $scan
    )
  )
  
  ;; 2. Sweep Phase
  global.get $allocated_list
  local.set $curr
  i32.const 0
  local.set $prev
  
  (block $done_sweep
    (loop $sweep
      local.get $curr
      i32.eqz
      br_if $done_sweep
      
      local.get $curr
      i32.load ;; Load next
      local.set $next
      
      ;; Check mark bit (offset 8)
      local.get $curr
      i32.const 8
      i32.add
      i32.load
      local.set $marked
      
      local.get $marked
      if
        ;; Marked: Keep it, unmark for next time
        local.get $curr
        i32.const 8
        i32.add
        i32.const 0
        i32.store
        
        local.get $curr
        local.set $prev
      else
        ;; Unmarked: Free it
        ;; Remove from allocated list
        local.get $prev
        i32.eqz
        if
          local.get $next
          global.set $allocated_list
        else
          local.get $prev
          local.get $next
          i32.store
        end
        
        ;; Add to free list (Reuse existing free logic logic simplified)
        ;; For now, we just add to free list without coalescing for simplicity
        local.get $curr
        i32.const 8 ;; Reuse mark/next_free slot for free list next
        i32.add
        global.get $free_list
        i32.store
        
        local.get $curr
        global.set $free_list
      end
      
      local.get $next
      local.set $curr
      br $sweep
    )
  )
)

(func $free (param $ptr i32)
  ;; No-op in GC world, or manual free
)

(func $strlen (param $str i32) (result i32)
  (local $len i32)
  (local $ptr i32)
  local.get $str
  local.set $ptr
  (block $break
    (loop $top
      local.get $ptr
      i32.load8_u
      i32.eqz
      br_if $break
      local.get $len
      i32.const 1
      i32.add
      local.set $len
      local.get $ptr
      i32.const 1
      i32.add
      local.set $ptr
      br $top
    )
  )
  local.get $len
)

(func $str_concat (param $s1 i32) (param $s2 i32) (result i32)
  (local $len1 i32)
  (local $len2 i32)
  (local $new_ptr i32)
  (local $dest i32)
  (local $src i32)
  local.get $s1
  call $strlen
  local.set $len1
  local.get $s2
  call $strlen
  local.set $len2
  local.get $len1
  local.get $len2
  i32.add
  i32.const 1
  i32.add
  i32.const 0 ;; TypeID 0 (String)
  call $malloc
  local.set $new_ptr
  local.get $new_ptr
  local.set $dest
  local.get $s1
  local.set $src
  (block $b1 (loop $l1
     local.get $src
     i32.load8_u
     i32.eqz
     br_if $b1
     local.get $dest
     local.get $src
     i32.load8_u
     i32.store8
     local.get $dest
     i32.const 1
     i32.add
     local.set $dest
     local.get $src
     i32.const 1
     i32.add
     local.set $src
     br $l1
  ))
  local.get $s2
  local.set $src
  (block $b2 (loop $l2
     local.get $src
     i32.load8_u
     i32.eqz
     br_if $b2
     local.get $dest
     local.get $src
     i32.load8_u
     i32.store8
     local.get $dest
     i32.const 1
     i32.add
     local.set $dest
     local.get $src
     i32.const 1
     i32.add
     local.set $src
     br $l2
  ))
  local.get $dest
  i32.const 0
  i32.store8
  local.get $new_ptr
)
`

type Symbol struct {
	Index   int
	Type    DataType
	IsParam bool
	ShadowIndex int // Index in the shadow stack (-1 if not tracked)
}

// FunctionScope represents a function being compiled
type FunctionScope struct {
	Name         string
	Instructions []string
	Symbols      map[string]Symbol
	NextLocalID  int
	ParamCount   int
	ParamTypes   []DataType
	ShadowStackSize int // Number of pointer locals tracked
}

type ClassSymbol struct {
	Name       string
	Size       int
	Fields     map[string]int      // Name -> Offset
	FieldTypes map[string]DataType // Name -> Type
	Methods    map[string]string   // Name -> MangledName
	Parent     string              // Parent class name (empty if none)
	TypeID     int                 // Unique Type ID for GC
}

func NewFunctionScope(name string) *FunctionScope {
	return &FunctionScope{
		Name:         name,
		Instructions: []string{},
		Symbols:      make(map[string]Symbol),
		NextLocalID:  0,
		ParamCount:   0,
		ParamTypes:   []DataType{},
	}
}

// Compiler converts AST to WAT (WebAssembly Text Format)
type Compiler struct {
	functions      []*FunctionScope
	current        *FunctionScope
	imports        []string
	stringPool     map[string]int // String literal -> memory offset
	nextDataOffset int            // Next available memory offset
	classes        map[string]ClassSymbol // Class name -> Symbol
	currentClass   string                 // Current class being compiled
	nextTypeID     int                    // Next available Type ID (start from 2)

	// Type checking state
	stackType DataType
}

func New() *Compiler {
	return &Compiler{
		functions:      []*FunctionScope{},
		imports:        []string{},
		stringPool:     make(map[string]int),
		nextDataOffset: 0,
		classes:        make(map[string]ClassSymbol),
		nextTypeID:     2, // 0=String/Primitive, 1=Array(TODO), 2+=Classes
	}
}

func (c *Compiler) Compile(node ast.Node) error {
	switch node := node.(type) {
	case *ast.Program:
		// 1. First pass: Compile imports
		for _, stmt := range node.Statements {
			if _, ok := stmt.(*ast.ImportStatement); ok {
				if err := c.Compile(stmt); err != nil {
					return err
				}
			}
		}

		// 1.5 Pass: Define Classes
		for _, stmt := range node.Statements {
			if classStmt, ok := stmt.(*ast.ClassStatement); ok {
				if err := c.defineClass(classStmt); err != nil {
					return err
				}
			}
		}

		// 1.6 Pass: Compile Class Methods
		for _, stmt := range node.Statements {
			if classStmt, ok := stmt.(*ast.ClassStatement); ok {
				if err := c.compileClassMethods(classStmt); err != nil {
					return err
				}
			}
		}

		// 2. Second pass: Compile functions
		for _, stmt := range node.Statements {
			if exprStmt, ok := stmt.(*ast.ExpressionStatement); ok {
				if fn, ok := exprStmt.Expression.(*ast.FunctionLiteral); ok {
					if err := c.compileFunction(fn); err != nil {
						return err
					}
				}
			}
		}

	case *ast.ImportStatement:
		// Generate Wasm import
		// MVP Assumption: All imported functions take one i32 and return void or i32
		funcName := node.Name.Value
		importStr := fmt.Sprintf("(import \"env\" \"%s\" (func $%s (param i32)))", funcName, funcName)
		c.imports = append(c.imports, importStr)
		
	case *ast.ExpressionStatement:
		if err := c.Compile(node.Expression); err != nil {
			return err
		}
		c.emit("drop")

	case *ast.LetStatement:
		if err := c.Compile(node.Value); err != nil {
			return err
		}

		valueType := c.stackType
		if valueType == TypeUnknown {
			valueType = TypeInt // Default to int for MVP if unknown
		}

		index := c.current.NextLocalID
		shadowIndex := c.current.ShadowStackSize
		
		c.current.Symbols[node.Name.Value] = Symbol{
			Index: index, 
			Type: valueType, 
			IsParam: false,
			ShadowIndex: shadowIndex,
		}
		c.current.NextLocalID++
		c.current.ShadowStackSize++

		realIndex := index + c.current.ParamCount
		// local.set %d
		c.emit(fmt.Sprintf("local.set %d ;; %s (%s)", realIndex, node.Name.Value, valueType))
		
		// Push to shadow stack
		// store(shadow_stack_ptr, value)
		// shadow_stack_ptr += 4
		c.emit("global.get $shadow_stack_ptr")
		c.emit(fmt.Sprintf("local.get %d", realIndex))
		c.emit("i32.store")
		
		c.emit("global.get $shadow_stack_ptr")
		c.emit("i32.const 4")
		c.emit("i32.add")
		c.emit("global.set $shadow_stack_ptr")

	case *ast.ClassStatement:
		// Already handled in passes
		return nil

	case *ast.NewExpression:
		className := node.Class.Value
		classSym, ok := c.classes[className]
		if !ok {
			return fmt.Errorf("undefined class: %s", className)
		}

		// malloc(size)
		c.emit(fmt.Sprintf("i32.const %d", classSym.Size))
		c.emit(fmt.Sprintf("i32.const %d", classSym.TypeID))
		c.emit("call $malloc")

		// Check for constructor "init"
		if mangledName, ok := classSym.Methods["init"]; ok {
			// Store ptr in temp to use it multiple times
			tempIndex := c.current.NextLocalID
			c.current.NextLocalID++
			realTempIndex := tempIndex + c.current.ParamCount

			c.emit(fmt.Sprintf("local.set %d", realTempIndex))

			// Prepare 'this'
			c.emit(fmt.Sprintf("local.get %d", realTempIndex))

			// Args
			for _, arg := range node.Arguments {
				if err := c.Compile(arg); err != nil {
					return err
				}
			}

			c.emit(fmt.Sprintf("call $%s", mangledName))
			c.emit("drop") // Ignore init return value

			// Return instance
			c.emit(fmt.Sprintf("local.get %d", realTempIndex))
		} else if len(node.Arguments) > 0 {
			return fmt.Errorf("arguments provided for class %s but no 'init' method found", className)
		}

		c.stackType = TypeInt
		return nil

	case *ast.SuperExpression:
		// super.method() logic needs MemberExpression context, but if used alone?
		// Usually super() is for constructor.
		// super.foo() is for method access.
		// AST structure for super.foo() is MemberExpression(Object=SuperExpression, Property=foo)
		
		// If we are here, it means 'super' is used as a value, which is not really valid in this MVP except for member access.
		// But let's return 'this' pointer because super calls usually operate on 'this' instance.
		c.emit("local.get 0 ;; this (super)")
		c.stackType = TypeInt
		return nil

	case *ast.MemberExpression:
		// Special handling for super.method()
		if _, ok := node.Object.(*ast.SuperExpression); ok {
			if c.currentClass == "" {
				return fmt.Errorf("super used outside of class")
			}
			// We just let it fall through. compile(SuperExpression) puts 'this' on stack.
			// If it's a field access, the generic field lookup below will find it (inherited fields are copied).
			// If it's a method access not in a call, it will fail or behave weirdly, but we don't support function pointers yet.
		}

		if err := c.Compile(node.Object); err != nil {
			return err
		}
		
		propName := node.Property.Value
		
		// MVP: Search for property in all known classes (since we lack full type system)
		// Better approach: Since we don't have type info on stack, we have to guess or search all classes.
		// If multiple classes have same field name but different types, we might have issues.
		// Ideally, we should track type of object on stack. But c.stackType is just DataType enum.
		// If c.stackType is TypeInt (pointer), we don't know which class it is.
		
		offset := -1
		var fieldType DataType = TypeInt
		
		found := false
		for _, cls := range c.classes {
			if off, ok := cls.Fields[propName]; ok {
				offset = off
				if ft, ok := cls.FieldTypes[propName]; ok {
					fieldType = ft
				}
				found = true
				break
			}
		}
		
		if !found {
			return fmt.Errorf("unknown property: %s", propName)
		}
		
		c.emit(fmt.Sprintf("i32.const %d", offset))
		c.emit("i32.add")
		c.emit("i32.load")
		c.stackType = fieldType
		return nil

	case *ast.AssignmentExpression:
		// Handle MemberExpression assignment: obj.prop = val
		if member, ok := node.Left.(*ast.MemberExpression); ok {
			if err := c.Compile(member.Object); err != nil {
				return err
			}
			
			propName := member.Property.Value
			offset := -1
			for _, cls := range c.classes {
				if off, ok := cls.Fields[propName]; ok {
					offset = off
					break
				}
			}
			if offset == -1 {
				return fmt.Errorf("unknown property in assignment: %s", propName)
			}
			
			c.emit(fmt.Sprintf("i32.const %d", offset))
			c.emit("i32.add")
			
			if err := c.Compile(node.Value); err != nil {
				return err
			}
			
			c.emit("i32.store")
			c.emit("i32.const 0") // Assignment returns 0 (void)
			return nil
		}
		
		// Handle Identifier assignment: x = val
		if ident, ok := node.Left.(*ast.Identifier); ok {
			if err := c.Compile(node.Value); err != nil {
				return err
			}
			
			sym, ok := c.current.Symbols[ident.Value]
			if !ok {
				return fmt.Errorf("undefined variable: %s", ident.Value)
			}
			
			realIndex := sym.Index
			if !sym.IsParam {
				realIndex += c.current.ParamCount
			}
			
			c.emit(fmt.Sprintf("local.set %d", realIndex))
			c.emit(fmt.Sprintf("local.get %d", realIndex))
			
			// Update shadow stack: value = stack[shadowBase + shadowIndex*4]
			// We have shadowBase in local(shadowBaseIndex).
			shadowBaseIndex := c.current.ParamCount
			
			c.emit(fmt.Sprintf("local.get %d ;; shadow base", shadowBaseIndex))
			c.emit(fmt.Sprintf("i32.const %d", sym.ShadowIndex * 4))
			c.emit("i32.add")
			c.emit(fmt.Sprintf("local.get %d ;; value", realIndex))
			c.emit("i32.store")
			
			c.emit("i32.const 0") // Assignment returns 0 (void)
			return nil
		}
		
		return fmt.Errorf("invalid assignment target")

	case *ast.ThisExpression:
		// 'this' is always param 0
		c.emit("local.get 0 ;; this")
		c.stackType = TypeInt
		return nil

	case *ast.Identifier:
		sym, ok := c.current.Symbols[node.Value]
		if !ok {
			return fmt.Errorf("undefined variable: %s", node.Value)
		}

		realIndex := sym.Index
		if !sym.IsParam {
			realIndex += c.current.ParamCount
		}

		c.emit(fmt.Sprintf("local.get %d ;; %s (%s)", realIndex, node.Value, sym.Type))
		c.stackType = sym.Type

	case *ast.InfixExpression:
		if err := c.Compile(node.Left); err != nil {
			return err
		}
		leftType := c.stackType

		if err := c.Compile(node.Right); err != nil {
			return err
		}
		rightType := c.stackType

		switch node.Operator {
		case "+":
			if leftType == TypeInt && rightType == TypeInt {
				c.emit("i32.add")
				c.stackType = TypeInt
			} else if leftType == TypeString && rightType == TypeString {
				c.emit("call $str_concat")
				c.stackType = TypeString
			} else {
				return fmt.Errorf("operator + not defined for types %s and %s", leftType, rightType)
			}
		case "-":
			if leftType == TypeInt && rightType == TypeInt {
				c.emit("i32.sub")
				c.stackType = TypeInt
			} else {
				return fmt.Errorf("operator - not defined for types %s and %s", leftType, rightType)
			}
		case "*":
			if leftType == TypeInt && rightType == TypeInt {
				c.emit("i32.mul")
				c.stackType = TypeInt
			} else {
				return fmt.Errorf("operator * not defined for types %s and %s", leftType, rightType)
			}
		case "/":
			if leftType == TypeInt && rightType == TypeInt {
				c.emit("i32.div_s")
				c.stackType = TypeInt
			} else {
				return fmt.Errorf("operator / not defined for types %s and %s", leftType, rightType)
			}
		case "==":
			if leftType == rightType {
				c.emit("i32.eq")
				c.stackType = TypeBool
			} else {
				return fmt.Errorf("operator == not defined for types %s and %s", leftType, rightType)
			}
		case "!=":
			if leftType == rightType {
				c.emit("i32.ne")
				c.stackType = TypeBool
			} else {
				return fmt.Errorf("operator != not defined for types %s and %s", leftType, rightType)
			}
		case "<":
			if leftType == TypeInt && rightType == TypeInt {
				c.emit("i32.lt_s")
				c.stackType = TypeBool
			} else {
				return fmt.Errorf("operator < not defined for types %s and %s", leftType, rightType)
			}
		case ">":
			if leftType == TypeInt && rightType == TypeInt {
				c.emit("i32.gt_s")
				c.stackType = TypeBool
			} else {
				return fmt.Errorf("operator > not defined for types %s and %s", leftType, rightType)
			}
		default:
			return fmt.Errorf("unknown operator %s", node.Operator)
		}

	case *ast.CallExpression:
		if member, ok := node.Function.(*ast.MemberExpression); ok {
			// Check for super.method()
			if _, isSuper := member.Object.(*ast.SuperExpression); isSuper {
				if c.currentClass == "" {
					return fmt.Errorf("super call outside of class method")
				}
				
				currentSym := c.classes[c.currentClass]
				parentName := currentSym.Parent
				if parentName == "" {
					return fmt.Errorf("super call in class with no parent")
				}
				
				parentSym := c.classes[parentName]
				methodName := member.Property.Value
				
				mangledName, ok := parentSym.Methods[methodName]
				if !ok {
					return fmt.Errorf("method %s not found in parent class %s", methodName, parentName)
				}
				
				// Push 'this' (local 0) as first argument
				c.emit("local.get 0 ;; this (super)")
				
				// Compile other arguments
				for _, arg := range node.Arguments {
					if err := c.Compile(arg); err != nil {
						return err
					}
				}
				
				c.emit(fmt.Sprintf("call $%s", mangledName))
				c.stackType = TypeInt
				return nil
			}

			// Method call: obj.method(args)
			// 1. Compile obj to get 'this' pointer
			if err := c.Compile(member.Object); err != nil {
				return err
			}
			// Stack: [obj_ptr]

			// Look up method name in ALL classes.
			methodName := member.Property.Value
			var mangledName string
			found := false
			for _, cls := range c.classes {
				if name, ok := cls.Methods[methodName]; ok {
					mangledName = name
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("unknown method: %s", methodName)
			}
			
			// Compile other arguments
			for _, arg := range node.Arguments {
				if err := c.Compile(arg); err != nil {
					return err
				}
			}
			
			c.emit(fmt.Sprintf("call $%s", mangledName))
			c.stackType = TypeInt
			return nil
		}

		for _, arg := range node.Arguments {
			if err := c.Compile(arg); err != nil {
				return err
			}
		}

		if ident, ok := node.Function.(*ast.Identifier); ok {
			c.emit(fmt.Sprintf("call $%s", ident.Value))
		} else {
			return fmt.Errorf("complex function calls not supported yet")
		}
		// MVP: Assume functions return int
		c.stackType = TypeInt

	case *ast.IntegerLiteral:
		c.emit(fmt.Sprintf("i32.const %d", node.Value))
		c.stackType = TypeInt

	case *ast.StringLiteral:
		offset, ok := c.stringPool[node.Value]
		if !ok {
			offset = c.nextDataOffset
			c.stringPool[node.Value] = offset
			c.nextDataOffset += len(node.Value) + 1
		}
		c.emit(fmt.Sprintf("i32.const %d ;; pointer to \"%s\"", offset, node.Value))
		c.stackType = TypeString

	case *ast.ArrayLiteral:
		length := len(node.Elements)
		size := 4 + length*4 // header(4) + elements

		c.emit(fmt.Sprintf("i32.const %d", size))
		c.emit("i32.const 1") // TypeID 1 (Array - TODO: Implement trace for arrays)
		c.emit("call $malloc")

		// Use a temporary local to store the pointer
		tempName := fmt.Sprintf("$arr_temp_%d", c.current.NextLocalID)
		tempIndex := c.current.NextLocalID
		c.current.Symbols[tempName] = Symbol{Index: tempIndex, Type: TypeArray, IsParam: false}
		c.current.NextLocalID++

		c.emit(fmt.Sprintf("local.tee %d", tempIndex+c.current.ParamCount))

		// Store length
		c.emit(fmt.Sprintf("i32.const %d", length))
		c.emit("i32.store")

		// Store elements
		for i, el := range node.Elements {
			// Calculate address: ptr + 4 + i*4
			c.emit(fmt.Sprintf("local.get %d", tempIndex+c.current.ParamCount))
			c.emit(fmt.Sprintf("i32.const %d", 4+i*4))
			c.emit("i32.add")

			if err := c.Compile(el); err != nil {
				return err
			}

			// Store value
			c.emit("i32.store")
		}

		// Return pointer
		c.emit(fmt.Sprintf("local.get %d", tempIndex+c.current.ParamCount))
		c.stackType = TypeArray

	case *ast.IndexExpression:
		if err := c.Compile(node.Left); err != nil {
			return err
		}
		// check if left is array
		// if c.stackType != TypeArray { return fmt.Errorf("index operation on non-array") }

		if err := c.Compile(node.Index); err != nil {
			return err
		}

		// ptr + 4 + index * 4
		c.emit("i32.const 4")
		c.emit("i32.mul")
		c.emit("i32.add")
		c.emit("i32.const 4")
		c.emit("i32.add")

		c.emit("i32.load")
		c.stackType = TypeInt // Assume array elements are int for now

	case *ast.Boolean:
		if node.Value {
			c.emit("i32.const 1")
		} else {
			c.emit("i32.const 0")
		}
		c.stackType = TypeBool

	case *ast.IfExpression:
		if err := c.Compile(node.Condition); err != nil {
			return err
		}

		c.emit("if (result i32)")

		if err := c.Compile(node.Consequence); err != nil {
			return err
		}

		c.emit("else")
		if node.Alternative != nil {
			if err := c.Compile(node.Alternative); err != nil {
				return err
			}
		} else {
			// Dummy return for else branch to satisfy type checker
			c.emit("i32.const 0")
		}

		c.emit("end")

	case *ast.FunctionLiteral:
		// Should be handled by compileFunction called from Program
		return nil

	case *ast.BlockStatement:
		for _, stmt := range node.Statements {
			if err := c.Compile(stmt); err != nil {
				return err
			}
		}

	case *ast.ReturnStatement:
		if err := c.Compile(node.ReturnValue); err != nil {
			return err
		}
		c.emit("return")

	case *ast.WhileStatement:
		c.emit("block $break")
		c.emit("loop $continue")

		// Compile condition
		if err := c.Compile(node.Condition); err != nil {
			return err
		}

		// Check condition: if false (0), break
		c.emit("i32.eqz")
		c.emit("br_if $break")

		// Compile body
		if err := c.Compile(node.Body); err != nil {
			return err
		}

		// Jump back to start of loop
		c.emit("br $continue")

		c.emit("end") // end of loop
		c.emit("end") // end of block
	}
	return nil
}

func (c *Compiler) compileFunction(fn *ast.FunctionLiteral) error {
	scope := NewFunctionScope(fn.Name)
	c.current = scope
	c.functions = append(c.functions, scope)

	// Save previous shadow stack pointer
	// We need a local for this
	shadowPtrLocal := scope.NextLocalID
	scope.NextLocalID++
	c.emit("global.get $shadow_stack_ptr")
	c.emit(fmt.Sprintf("local.set %d ;; save previous shadow_stack_ptr", shadowPtrLocal))

	for i, param := range fn.Parameters {
		// MVP: Assume all parameters are int (but some might be objects)
		// For now, assume everything is an object/pointer for simplicity in shadow stack?
		// Or assume int?
		// If we mark integers as roots, it's fine (conservative GC), but they might look like pointers.
		// For safety, let's assume everything is tracked in shadow stack for now.
		
		scope.Symbols[param.Value] = Symbol{
			Index: i, 
			Type: TypeInt, 
			IsParam: true,
			ShadowIndex: scope.ShadowStackSize,
		}
		scope.ParamTypes = append(scope.ParamTypes, TypeInt)
		scope.ParamCount++
		
		// Push param to shadow stack
		// global.set $shadow_stack_ptr (ptr + 4)
		// i32.store (ptr, param)
		c.emit("global.get $shadow_stack_ptr")
		c.emit(fmt.Sprintf("local.get %d", i))
		c.emit("i32.store")
		
		c.emit("global.get $shadow_stack_ptr")
		c.emit("i32.const 4")
		c.emit("i32.add")
		c.emit("global.set $shadow_stack_ptr")
		
		scope.ShadowStackSize++
	}

	if err := c.Compile(fn.Body); err != nil {
		return err
	}

	// Restore shadow stack pointer
	c.emit(fmt.Sprintf("local.get %d", shadowPtrLocal))
	c.emit("global.set $shadow_stack_ptr")

	// Implicit return 0 if no return statement (for void functions or just safety)
	c.emit("i32.const 0")
	return nil
}

func (c *Compiler) emit(instruction string) {
	if c.current != nil {
		c.current.Instructions = append(c.current.Instructions, instruction)
	}
}

// GenerateWAT returns the final WAT string
func (c *Compiler) GenerateWAT() string {
	var out bytes.Buffer
	out.WriteString("(module\n")

	// Imports must come first
	for _, imp := range c.imports {
		out.WriteString("  " + imp + "\n")
	}

	// Export memory so host can access it
	out.WriteString("  (memory (export \"memory\") 1)\n")
	
	// Export GC
	out.WriteString("  (export \"gc\" (func $gc_collect))\n")

	// Emit data segments for strings
	for str, offset := range c.stringPool {
		// Basic escaping for WAT
		escapedStr := fmt.Sprintf("%q", str)
		escapedStr = escapedStr[1 : len(escapedStr)-1] // Remove Go quotes
		out.WriteString(fmt.Sprintf("  (data (i32.const %d) \"%s\\00\")\n", offset, escapedStr))
	}

	// Emit Standard Library
	out.WriteString(stdLibWAT)

	// Emit GC Trace Function
	c.emitGCTrace(&out)

	for _, fn := range c.functions {
		exportName := ""
		if fn.Name == "main" {
			exportName = "(export \"main\")"
		}

		paramsStr := ""
		for i := 0; i < fn.ParamCount; i++ {
			paramsStr += " (param i32)"
		}

		out.WriteString(fmt.Sprintf("  (func $%s %s%s (result i32)\n", fn.Name, exportName, paramsStr))

		for i := 0; i < fn.NextLocalID; i++ {
			out.WriteString("    (local i32)\n")
		}

		for _, ins := range fn.Instructions {
			out.WriteString("    " + ins + "\n")
		}

		out.WriteString("  )\n")
	}

	out.WriteString(")\n")
	return out.String()
}

func (c *Compiler) emitGCTrace(out *bytes.Buffer) {
	out.WriteString("(func $gc_trace (param $ptr i32) (param $type_id i32)\n")
	
	// Create block for each class + default
	// br_table [default, array, class1, class2...]
	// TypeID: 0=String(ignore), 1=Array(TODO), 2=Class1, 3=Class2...
	
	// We need to sort classes by TypeID to generate br_table or if/else chain
	// Since we don't have many classes yet, if/else chain is fine and easier.
	
	for _, cls := range c.classes {
		out.WriteString(fmt.Sprintf("  ;; Class %s (TypeID %d)\n", cls.Name, cls.TypeID))
		out.WriteString("  local.get $type_id\n")
		out.WriteString(fmt.Sprintf("  i32.const %d\n", cls.TypeID))
		out.WriteString("  i32.eq\n")
		out.WriteString("  if\n")
		
		// Scan fields
		for name, offset := range cls.Fields {
			fieldType := cls.FieldTypes[name]
			// Only trace pointer types
			// Assuming Int/Bool are values. String is pointer but leaf (no trace inside).
			// Array and Class types need tracing.
			// How do we know if a field is a Class type?
			// DataType is string. If it's not int/bool/string/void, it's a class or array.
			
			if fieldType != TypeInt && fieldType != TypeBool && fieldType != TypeString && fieldType != TypeVoid {
				out.WriteString(fmt.Sprintf("    ;; Field %s (offset %d)\n", name, offset))
				out.WriteString("    local.get $ptr\n")
				out.WriteString(fmt.Sprintf("    i32.const %d\n", offset))
				out.WriteString("    i32.add\n")
				out.WriteString("    i32.load\n")
				out.WriteString("    call $gc_mark\n")
			}
		}
		
		out.WriteString("    return\n")
		out.WriteString("  end\n")
	}
	
	out.WriteString(")\n")
}

func (c *Compiler) defineClass(node *ast.ClassStatement) error {
	className := node.Name.Value
	classSymbol := ClassSymbol{
		Name:       className,
		Fields:     make(map[string]int),
		FieldTypes: make(map[string]DataType),
		Methods:    make(map[string]string),
		TypeID:     c.nextTypeID,
	}
	c.nextTypeID++

	offset := 0

	// Inherit from parent
	if node.Parent != nil {
		parentName := node.Parent.Value
		parentSym, ok := c.classes[parentName]
		if !ok {
			return fmt.Errorf("undefined parent class: %s", parentName)
		}
		classSymbol.Parent = parentName
		
		// Copy fields
		for k, v := range parentSym.Fields {
			classSymbol.Fields[k] = v
		}
		for k, v := range parentSym.FieldTypes {
			classSymbol.FieldTypes[k] = v
		}
		offset = parentSym.Size
		
		// Copy methods
		for k, v := range parentSym.Methods {
			classSymbol.Methods[k] = v
		}
	}

	// Add new fields
	for _, field := range node.Fields {
		classSymbol.Fields[field.Name.Value] = offset
		
		// Parse type
		typeName := field.Type
		dataType := TypeInt // Default
		if typeName == "string" {
			dataType = TypeString
		} else if typeName == "int" {
			dataType = TypeInt
		} else if typeName == "bool" {
			dataType = TypeBool
		} else if typeName == "array" {
			dataType = TypeArray
		} else if typeName != "" {
			// For now, treat other types as pointers (int)
			dataType = TypeInt
		}
		classSymbol.FieldTypes[field.Name.Value] = dataType
		
		offset += 4 
	}
	classSymbol.Size = offset

	// Add/Override methods
	for _, method := range node.Methods {
		mangledName := fmt.Sprintf("%s_%s", className, method.Name)
		classSymbol.Methods[method.Name] = mangledName
	}

	c.classes[className] = classSymbol
	return nil
}

func (c *Compiler) compileClassMethods(node *ast.ClassStatement) error {
	className := node.Name.Value
	c.currentClass = className
	defer func() { c.currentClass = "" }()

	for _, method := range node.Methods {
		mangledName := fmt.Sprintf("%s_%s", className, method.Name)

		scope := NewFunctionScope(mangledName)
		c.current = scope
		c.functions = append(c.functions, scope)

		// Add 'this' parameter as first parameter
		scope.Symbols["this"] = Symbol{Index: 0, Type: TypeInt, IsParam: true}
		scope.ParamTypes = append(scope.ParamTypes, TypeInt)
		scope.ParamCount++

		for i, param := range method.Parameters {
			scope.Symbols[param.Value] = Symbol{Index: i + 1, Type: TypeInt, IsParam: true}
			scope.ParamTypes = append(scope.ParamTypes, TypeInt)
			scope.ParamCount++
		}

		if err := c.Compile(method.Body); err != nil {
			return err
		}

		c.emit("i32.const 0")
	}
	return nil
}
