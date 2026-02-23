package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"omniScript/pkg/compiler"
	"omniScript/pkg/lexer"
	"omniScript/pkg/parser"
)

func main() {
	target := flag.String("target", "browser", "Target platform (browser or wasi)")
	flag.Parse()
	args := flag.Args()

	if len(args) < 1 {
		fmt.Println("Usage: omni <filename.omni>")
		os.Exit(1)
	}

	filename := args[0]
	fmt.Printf("Compiling %s...\n", filename)

	content, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	// 1. Lexing
	l := lexer.New(string(content))
	
	// 2. Parsing
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) != 0 {
		fmt.Println("Parser errors:")
		for _, msg := range p.Errors() {
			fmt.Println("\t" + msg)
		}
		os.Exit(1)
	}

	// 3. Compiling
	c := compiler.New(*target)
	if err := c.Compile(program); err != nil {
		fmt.Printf("Compiler error: %v\n", err)
		os.Exit(1)
	}

	// 4. Output
	watContent := c.GenerateWAT()
	
	outputDir := "output"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	baseName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	outputFile := filepath.Join(outputDir, baseName+".wat")
	
	if err := os.WriteFile(outputFile, []byte(watContent), 0644); err != nil {
		fmt.Printf("Error writing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Success! Generated %s\n", outputFile)
	fmt.Println("You can verify it online at: https://webassembly.github.io/wabt/demo/wat2wasm/")
}