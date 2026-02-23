# OmniScript

OmniScript is a unified programming language designed to bridge the gap between high-performance backend systems and modern frontend web applications. It aims to combine the ease of use of TypeScript with the raw performance and concurrency model of Go, all within a single language ecosystem.

## Vision

The core philosophy of OmniScript is "One Language, Two Worlds":

1.  **Frontend (WebAssembly)**:
    - Compiles to optimized WebAssembly (Wasm).
    - **Native Web APIs**: Direct access to `window`, `document`, `canvas`, etc., without heavy bindings.
    - **Automatic Concurrency**: The compiler analyzes code to detect compute-intensive tasks and automatically offloads them to **Web Workers**.
    - **Zero-Copy Data Sharing**: Uses `SharedArrayBuffer` and `Atomics` for efficient data sharing between the main thread and workers, eliminating the overhead of message passing.
    - Provides a TypeScript-like development experience.

2.  **Backend (Native)**:
    - Compiles to native binary executables (like Go/Rust).
    - **High Concurrency**: Built-in support for lightweight threads (coroutines/goroutines) for handling massive concurrent connections.
    - **High Efficiency**: AOT (Ahead-of-Time) compilation with manual memory management options or efficient GC, suitable for system-level programming.
    - Bridges the gap between high-level logic and low-level system access.

## Features Status

### Implemented

- **Compiler Core**:
  - [x] Lexer & Parser (TypeScript-like syntax)
  - [x] AST Generation
  - [x] WAT (WebAssembly Text) Code Generation for MVP
  - [x] CLI Tool (`omni`)

- **Type System**:
  - [x] Basic Types: `int` (i32), `bool`, `void`
  - [x] Strings: Immutable, string pool, concatenation
  - [x] Arrays: Dynamic arrays with bounds checking
  - [x] Maps/Objects: Hash maps, string keys, index access
  - [x] Classes: Properties, methods, single inheritance, polymorphism

- **Memory Management**:
  - [x] Memory Allocator (`malloc`/`free`)
  - [x] Garbage Collection (Mark-and-Sweep)

### Roadmap

#### Frontend (Wasm + Browser)
- [ ] **DOM & Web APIs**: First-class support for `window`, `document`, `canvas`.
- [ ] **Auto-Parallelism**: Static analysis to identify CPU-bound loops/functions and generate Web Worker scaffolding.
- [ ] **Shared Memory**: Implementation of `SharedArrayBuffer` based heap and `Atomics` for synchronization.

#### Backend (Native)
- [ ] **Native Compilation**: LLVM or native assembly backend for generating `.exe`/elf binaries.
- [ ] **Concurrency Model**: M:N scheduler for lightweight threads (similar to Go routines).
- [ ] **System APIs**: File system, Networking (TCP/UDP/HTTP), Process management.

#### Language Core
- [ ] **Floats**: `float`/`f64` support.
- [ ] **Modules**: `import`/`export` system.
- [ ] **Error Handling**: `try`/`catch`/`throw`.
- [ ] **Generics**: Type parameters for flexible data structures.

## Getting Started

1. **Build the Compiler**:
   ```bash
   go build -o omni.exe cmd/omni/main.go
   ```

2. **Compile a Script**:
   ```bash
   ./omni examples/map_test.omni
   ```

3. **Run**:
   Currently targets WebAssembly (WAT). Use a Wasm runtime or browser to execute.

## Example Vision

```typescript
// Frontend: Direct DOM manipulation
function setupUI() {
    let btn = document.getElementById("myBtn");
    btn.onClick(() => {
        // Compute-intensive task automatically runs in a Worker
        let result = heavyComputation(1000);
        console.log("Result: " + result);
    });
}

// Compute-intensive function (Compiler marks this for Worker execution)
function heavyComputation(n: int): int {
    // ... complex math ...
    return n * n;
}

// Backend: High-concurrency server
function main() {
    let server = new HTTPServer();
    server.handle("/", (req, res) => {
        res.send("Hello form OmniScript Backend!");
    });
    server.listen(8080);
}
```
