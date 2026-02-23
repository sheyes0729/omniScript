# OmniScript

[English](./README.md)

OmniScript 是一门统一的编程语言，旨在弥合高性能后端系统与现代前端 Web 应用之间的鸿沟。它的目标是在单一的语言生态系统中，结合 TypeScript 的易用性与 Go 的高性能并发模型。

## 愿景 (Vision)

OmniScript 的核心理念是 **"一种语言，两个世界"**：

1.  **前端 (WebAssembly)**：
    - 编译为优化的 WebAssembly (Wasm)。
    - **原生 Web API**：无需繁重的绑定，直接在语言层面支持 `window`、`document`、`canvas` 等浏览器 API。
    - **自动并发**：编译器通过静态分析识别计算密集型任务，并自动将其卸载到 **Web Workers** 中执行。
    - **零拷贝数据共享**：基于 `SharedArrayBuffer` 和 `Atomics` 实现主线程与 Worker 之间的高效数据共享，消除传统消息传递的开销。
    - 提供类 TypeScript 的开发体验。

2.  **后端 (Native)**：
    - 编译为原生二进制可执行文件（类似 Go/Rust，支持 .exe/elf）。
    - **高并发**：内置支持 M:N 调度的轻量级线程（类似 Goroutines），轻松处理海量并发连接。
    - **高效率**：采用 AOT（提前编译）技术，结合可选的手动内存管理或高效 GC，适用于系统级编程。
    - 弥合高层业务逻辑与底层系统访问之间的差距。

## 开发路线图与状态 (Development Roadmap & Status)

### 📅 阶段 1：后端基础 ("Node.js Killer" 的起点)
**目标**：使 OmniScript 能够作为独立的后端应用程序运行，并与操作系统交互。

- [x] **多目标编译器**：支持 `-target=wasi` (后端) 和 `-target=browser` (前端)。
- [x] **WASI 集成**：实现 `wasi_snapshot_preview1` 绑定。
    - [x] 将 `console.log` 替换为 `fd_write` (stdout)。
    - [x] 读取命令行参数。
    - [x] 读取环境变量 (process.env)。
- [x] **文件系统 API**：实现 `std/fs` (与 Node.js 一致)。
    - [x] `fs.writeFile` / `fs.writeFileSync`.
    - [x] `fs.readFile` / `fs.readFileSync`.
    - [x] `fs.unlinkSync`, `fs.mkdirSync`, `fs.rmdirSync`, `fs.existsSync`.
    -   直接映射到 WASI `path_open`, `fd_read`, `fd_write`.
- [x] **控制流**：实现 `for` 和 `while` 循环。
- [x] **运行时 Shim**：一个轻量级的 Node.js 加载器，用于在开发期间在本地运行 OmniScript WASI 二进制文件 (`scripts/run_wasi.js`)。

### 📅 阶段 2：类型系统与核心语言特性 ("TypeScript" 的承诺)
**目标**：强制执行严格的类型安全并提供丰富的语言特性。

- [x] **类与继承**：支持 `class`, `extends`, `super`, `new`。
- [x] **接口**：支持 `interface` 定义结构化类型。
- [x] **枚举**：支持带有整数值的 `enum` 定义。
- [x] **类型别名**：支持 `type MyType = int`。
- [x] **编译时检查**：严格验证函数参数（数量）。
- [x] **泛型**：为函数和类实现 `<T>` (例如 `Array<T>`)。
- [ ] **高级类型**：联合类型 (`int | string`)。

### 📅 阶段 3：并发、内存与性能 ("Go" 的力量)
**目标**：解锁多核性能、自动任务分发和内存安全。

- [x] **内存管理**：
    - [x] **垃圾回收**：针对对象和数组的标记-清除 GC。
    - [x] **共享内存**：将默认内存模型切换为 `SharedArrayBuffer` (Wasm 线程)。
- [x] **并发**：
    - [x] **原子操作**：实现用于线程安全操作的 `Atomics` 内在函数。
    - [x] **`spawn` 关键字**：轻量级线程创建（分配新栈，共享堆）。
- [ ] **自动并行化**：
    - [ ] **任务调度器**：运行时逻辑，将任务分发给 Web Workers (前端) 或系统线程 (后端)。
    - [ ] **计算密度分析**：编译器分析过程，用于识别“繁重”的函数以便自动卸载。

### 📅 阶段 4：标准库与生态系统
**目标**：提供“电池包含”的模块，以实现快速开发。

- [x] **核心数据结构**：
    - [x] **数组**：动态数组 (`[]`, `.push`, `.length`)。
    - [x] **映射**：哈希映射 (`{}`, 键访问)。
    - [x] **字符串**：基本的字符串操作 (`substring`, `charCodeAt`, `length`)。
- [x] **std/path**：跨平台路径操作。
- [x] **std/fs**：文件系统 API (见阶段 1)。
- [ ] **std/http**：高性能 HTTP 1.1/2 服务器 (使用 WASI-socket 或主机绑定)。
- [ ] **std/net**：低级 TCP/UDP 访问。
- [x] **std/os**：操作系统级交互 (process.exit, process.env)。

---

### 下一步计划 (Next Immediate Steps)
1.  实现 **高级类型** (联合类型)。
2.  开始 **std/http** 实现。
3.  扩展 **std/os** 以包含更多系统调用。

## 快速开始 (Getting Started)

1. **构建编译器**:
   ```bash
   go build -o omni.exe cmd/omni/main.go
   ```

2. **编译脚本**:
   ```bash
   ./omni examples/map_test.omni
   ```

3. **运行**:
   当前目标平台为 WebAssembly (WAT)。请使用 Wasm 运行时或浏览器执行生成的代码。

## 愿景示例 (Example Vision)

```typescript
// 前端：直接操作 DOM
function setupUI() {
    let btn = document.getElementById("myBtn");
    btn.onClick(() => {
        // 计算密集型任务自动在 Worker 中运行
        let result = heavyComputation(1000);
        console.log("Result: " + result);
    });
}

// 计算密集型函数（编译器标记为 Worker 执行）
function heavyComputation(n: int): int {
    // ... 复杂的数学运算 ...
    return n * n;
}

// 后端：高并发服务器
function main() {
    let server = new HTTPServer();
    server.handle("/", (req, res) => {
        res.send("Hello from OmniScript Backend!");
    });
    server.listen(8080);
}
```
