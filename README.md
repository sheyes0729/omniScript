# OmniScript

[中文](./README.zh-CN.md)

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

## Development Roadmap & Status

### 📅 Phase 1: Backend Foundation (The "Node.js Killer" Start)
**Goal**: Enable OmniScript to run as a standalone backend application, interacting with the OS.

- [x] **Multi-Target Compiler**: Support `-target=wasi` (Backend) and `-target=browser` (Frontend).
- [x] **WASI Integration**: Implement `wasi_snapshot_preview1` bindings.
    - [x] Replace `console.log` with `fd_write` (stdout).
    - [x] Read command line arguments.
    - [x] Read environment variables (process.env).
- [x] **File System API**: Implement `std/fs` (Node.js-identical).
    - [x] `fs.writeFile` / `fs.writeFileSync`.
    - [x] `fs.readFile` / `fs.readFileSync`.
    - [x] `fs.unlinkSync`, `fs.mkdirSync`, `fs.rmdirSync`, `fs.existsSync`.
    -   Direct mapping to WASI `path_open`, `fd_read`, `fd_write`.
- [x] **Control Flow**: Implement `for` and `while` loops.
- [x] **Runtime Shim**: A lightweight Node.js loader to run OmniScript WASI binaries locally during development (`scripts/run_wasi.js`).

### 📅 Phase 2: Type System & Core Language Features (The "TypeScript" Promise)
**Goal**: Enforce strict type safety and provide rich language features.

- [x] **Classes & Inheritance**: Support `class`, `extends`, `super`, `new`.
- [x] **Interfaces**: Support `interface` definitions for structural typing.
- [x] **Enums**: Support `enum` definitions with integer values.
- [x] **Type Aliases**: Support `type MyType = int`.
- [x] **Compile-Time Checks**: Strict validation of function arguments (count).
- [x] **Generics**: Implement `<T>` for functions and classes (e.g., `Array<T>`).
- [ ] **Advanced Types**: Union types (`int | string`).

### 📅 Phase 3: Concurrency, Memory & Performance (The "Go" Power)
**Goal**: Unlock multi-core performance, automatic task distribution, and memory safety.

- [x] **Memory Management**:
    - [x] **Garbage Collection**: Mark-and-Sweep GC for Objects and Arrays.
    - [x] **Shared Memory**: Switch default memory model to `SharedArrayBuffer` (Wasm Threads).
- [x] **Concurrency**:
    - [x] **Atomics**: Implement `Atomics` intrinsics for thread-safe operations.
    - [x] **`spawn` keyword**: Lightweight thread creation (allocates new Stack, shares Heap).
- [ ] **Auto-Parallelism**:
    - [ ] **Task Scheduler**: Runtime logic to distribute tasks to Web Workers (Frontend) or System Threads (Backend).
    - [ ] **Compute Density Analysis**: Compiler pass to identify "heavy" functions for auto-offloading.

### 📅 Phase 4: Standard Library & Ecosystem
**Goal**: Provide batteries-included modules for rapid development.

- [x] **Core Data Structures**:
    - [x] **Arrays**: Dynamic arrays (`[]`, `.push`, `.length`).
    - [x] **Maps**: Hash maps (`{}`, key access).
    - [x] **Strings**: Basic string manipulation (`substring`, `charCodeAt`, `length`).
- [x] **std/path**: Cross-platform path manipulation.
- [x] **std/fs**: File System API (see Phase 1).
- [ ] **std/http**: High-performance HTTP 1.1/2 Server (using WASI-socket or host bindings).
- [ ] **std/net**: Low-level TCP/UDP access.
- [x] **std/os**: OS-level interaction (process.exit, process.env).

---

### Next Immediate Steps
1.  Implement **Advanced Types** (Union Types).
2.  Start **std/http** implementation.
3.  Expand **std/os** with more system calls.

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
