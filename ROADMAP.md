# OmniScript Development Roadmap

## Vision
**"TypeScript Syntax, Go Performance, Node.js Versatility"**
OmniScript aims to be the ultimate full-stack language, enabling frontend developers to build high-performance backend services and complex frontend applications without learning a new syntax.

---

## 📅 Phase 1: Backend Foundation (The "Node.js Killer" Start)
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
    - [ ]  Direct mapping to WASI `path_open`, `fd_read`, `fd_write`.
- [x] **Control Flow**: Implement `for` loops.
- [x] **Runtime Shim**: A lightweight Node.js loader to run OmniScript WASI binaries locally during development (`scripts/run_wasi.js`).

## 📅 Phase 2: Type System Hardening (The "TypeScript" Promise)
**Goal**: Enforce strict type safety to prevent runtime errors.

- [x] **Interfaces**: Support `interface` definitions for structural typing.
- [ ] **Generics**: Implement `<T>` for functions and classes (e.g., `Array<T>`).
- [x] **Type Aliases**: Support `type MyType = int`.
- [ ] **Advanced Types**: Union types (`int | string`).
- [x] **Enums**: Support `enum` definitions with integer values.
- [x] **Compile-Time Checks**: Strict validation of function arguments (count).

## 📅 Phase 3: Concurrency & Performance (The "Go" Power)
**Goal**: Unlock multi-core performance and automatic task distribution.

- [x] **Shared Memory**: Switch default memory model to `SharedArrayBuffer` (Wasm Threads).
- [x] **Atomics**: Implement `Atomics` intrinsics for thread-safe operations.
- [ ] **Auto-Parallelism**:
    - [x] **`spawn` keyword**: Lightweight thread creation (allocates new Stack, shares Heap).
    - [ ] **Task Scheduler**: Runtime logic to distribute tasks to Web Workers (Frontend) or System Threads (Backend).
    - [ ] **Compute Density Analysis**: Compiler pass to identify "heavy" functions for auto-offloading.

## 📅 Phase 4: Standard Library & Ecosystem
**Goal**: Provide batteries-included modules for rapid development.

- [ ] **std/http**: High-performance HTTP 1.1/2 Server (using WASI-socket or host bindings).
- [ ] **std/net**: Low-level TCP/UDP access.
- [x] **std/path**: Cross-platform path manipulation.
- [ ] **std/os**: OS-level interaction (signals, user info).

---

## Next Immediate Steps
1.  Verify WASI execution with a runtime (e.g., Wasmtime or Node.js).
2.  Implement `std/fs` (File System) support via WASI.
3.  Implement basic Type System features (Interfaces).
