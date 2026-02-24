# OmniScript 开发全局规则

## 1. 核心定位与愿景
- 你正在辅助开发 **OmniScript (.omni)**：一门使用 Go 开发，但语法与 TypeScript (TS) 完全一致的高性能全栈语言。
- **目标**：零成本迁移 TS 代码，后端 API 必须与 Node.js (如 fs, http, path) 保持 100% 一致。
- **技术栈**：后端使用 Go (AOT 编译)，前端编译为 Wasm。

## 2. 交互约束
- **语言**：所有对话回复必须使用 **中文**。
- **记忆维持**：在处理任何复杂任务前，必须主动读取项目根目录的 `SPEC.md` 和 `README.md`。
- **防遗忘机制**：如果对话轮数过多，请主动提示用户：“已达到上下文阈值，我将总结当前进度并归档至 SPEC.md 以维持记忆。”

## 3. 开发规范
- **TS 兼容性**：解析器必须支持 TS 全量语法（interface, type, enum, generics 等）。
- **Node.js 兼容性**：定义 `omni:http` 等内置模块时，函数签名必须参考 Node.js 官方文档。
- **自动化流**：
    1. 完成代码编写后，必须检查并更新 `README.md` 中的 Todo List 进度。
    2. 自动执行代码质量自检，每个功能完成都要写测试代码。
    3. 代码测试通过后，帮我提交代码，并且git commit 的 message要符合Angular的提交信息规范。

## 4. Git 提交协议
- **严禁使用关键字**：提交信息（Commit Message）中禁止出现 `implement` / `scheduler` / `add` / `update`等等git定义的关键字，防止提交出现问题。
- **推荐格式**：
    - `feat: [desc]`
    - `fix: [desc]`
    - `refactor: [desc]`
    - `chore: [desc]`
    - `docs: [desc]`