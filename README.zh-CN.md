# OmniScript

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

## 功能状态 (Features Status)

### 已实现 (Implemented)

- **编译器核心**:
  - [x] 词法分析器与语法分析器（类 TypeScript 语法）
  - [x] AST 生成
  - [x] WAT (WebAssembly Text) 代码生成（MVP阶段）
  - [x] 命令行工具 (`omni`)

- **类型系统**:
  - [x] 基础类型：`int` (i32), `bool`, `void`
  - [x] 字符串：不可变、字符串池、拼接
  - [x] 数组：带边界检查的动态数组
  - [x] 映射/对象：哈希表、字符串键、索引访问
  - [x] 类：属性、方法、单继承、多态

- **内存管理**:
  - [x] 内存分配器 (`malloc`/`free`)
  - [x] 垃圾回收 (标记-清除算法)

### 路线图 (Roadmap)

#### 前端 (Wasm + Browser)
- [ ] **DOM & Web API**: 对 `window`、`document`、`canvas` 的一等公民支持。
- [ ] **自动并行化**: 静态分析 CPU 密集型循环/函数，自动生成 Web Worker 胶水代码。
- [ ] **共享内存**: 基于 `SharedArrayBuffer` 的堆实现及 `Atomics` 同步原语。

#### 后端 (Native)
- [ ] **原生编译**: 基于 LLVM 或原生汇编后端，生成 `.exe`/elf 二进制文件。
- [ ] **并发模型**: M:N 调度器实现的轻量级线程（协程）。
- [ ] **系统 API**: 文件系统、网络 (TCP/UDP/HTTP)、进程管理。

#### 语言核心
- [ ] **浮点数**: `float`/`f64` 支持。
- [ ] **模块化**: `import`/`export` 模块系统。
- [ ] **错误处理**: `try`/`catch`/`throw` 机制。
- [ ] **泛型**: 支持灵活数据结构的类型参数。

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
