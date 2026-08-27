<p align="center">
  <img src="build/appicon.png" width="72" height="72" alt="Claude Code 环境管理器" />
</p>

<h1 align="center">Claude Code 环境管理器</h1>

<p align="center">
  面向 Claude Code、Codex、Gemini CLI、OpenCode、Grok 的本地桌面工作台。<br />
  一处管理环境配置、MCP、Skills、本地 API 路由、监控轮换与云端备份。
</p>

<p align="center">
  <a href="./README.md"><strong>中文</strong></a> ·
  <a href="./README_EN.md">English</a>
</p>

<p align="center">
  <a href="https://github.com/nsmao-com/claude-code-env-change/releases"><img alt="Release" src="https://img.shields.io/github/v/release/nsmao-com/claude-code-env-change?style=flat-square" /></a>
  <a href="./LICENSE"><img alt="License" src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" /></a>
  <img alt="Go" src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square&logo=go&logoColor=white" />
  <img alt="Wails" src="https://img.shields.io/badge/Wails-v2-red?style=flat-square" />
  <img alt="Vue" src="https://img.shields.io/badge/Vue-3-42b883?style=flat-square&logo=vuedotjs&logoColor=white" />
  <img alt="Platform" src="https://img.shields.io/badge/platform-Windows-0078D4?style=flat-square&logo=windows&logoColor=white" />
</p>

<p align="center">
  <img src="portal.png" alt="应用界面" width="100%" />
</p>

## 这是什么

把四套 CLI 的环境变量、MCP 服务器、Skills、提示词和用量统计收进同一个原生窗口。配置写在本机，不经过第三方账号；需要换电脑时，可以用 S3 兼容对象存储加密备份。

当前版本 **v2.0.0**。

## 功能

| 模块 | 说明 |
| --- | --- |
| 环境 | 多配置、按平台筛选、拖拽排序、一键写入对应 CLI、延迟测速 |
| MCP | 管理 stdio / HTTP 服务器，同步到 Claude / Codex / Gemini |
| Skills | 编辑 `SKILL.md`，从内置技能库导入，按平台启用 |
| API 路由 | 本机 Anthropic ↔ OpenAI 网关，含 Codex Responses API |
| 监控 | 定时探测 Base URL，按轮换组自动切配置 |
| 云同步 | S3 / 阿里云 OSS / 兼容端点，AES-GCM 加密后上传 |
| 提示词 | 编辑各平台自定义系统提示词 |
| 统计 | 请求量、Token、花费估算、模型分布、活动热力图 |
| 更新 | 检测 GitHub Release，Windows 可在应用内下载并替换 |

## 安装

从 [Releases](https://github.com/nsmao-com/claude-code-env-change/releases) 下载 Windows 构建包后解压运行。

```
claude-env-switcher-windows-amd64.zip
```

系统需要 [WebView2](https://developer.microsoft.com/microsoft-edge/webview2/)。Windows 11 一般已自带。

macOS / Linux 可从源码构建，见下方。

## 从源码构建

**依赖**

- Go 1.22+
- Node.js 18+，包管理只用 **pnpm**
- [Wails v2 CLI](https://wails.io)

```bash
git clone https://github.com/nsmao-com/claude-code-env-change.git
cd claude-code-env-change
go install github.com/wailsapp/wails/v2/cmd/wails@latest
cd frontend && pnpm install && cd ..
wails dev
```

生产构建：

```bash
wails build
```

产物在 `build/bin/`。

## 数据放在哪

主配置目录：

```
~/.claude-env-switcher/
  config.json      环境配置
  mcp.json         MCP 服务器
  skills.json      Skills 索引
```

应用写入的 CLI 文件（按平台）：

| 平台 | 路径 |
| --- | --- |
| Claude Code | 系统环境变量 + Claude settings |
| Codex | `~/.codex/config.toml`、`~/.codex/auth.json` |
| Gemini CLI | `~/.gemini/.env`、`~/.gemini/settings.json` |
| OpenCode | `~/.config/opencode/opencode.json`（可用 `OPENCODE_CONFIG_DIR` / `OPENCODE_CONFIG` 覆盖路径） |
| Grok | `~/.grok/config.toml` |

旧版本若在启动目录留下了可写的 `config.json`，会继续使用该文件。

## 架构

```
┌─────────────────────────────────────────────┐
│  Vue 3  ·  Pinia  ·  shadcn-vue  ·  Tailwind 4 │
│  motion-v  ·  Chart.js                       │
└──────────────────────┬──────────────────────┘
                       │ Wails bindings
┌──────────────────────▼──────────────────────┐
│  Go  ·  Wails v2                             │
│  环境 / MCP / Skills / 路由网关               │
│  监控轮换 / OSS 云同步 / GitHub 更新          │
└─────────────────────────────────────────────┘
```

本地路由网关把 Anthropic Messages 与 OpenAI Chat Completions（含 Codex Responses）互相转换，让同一份上游 Key 给多套 CLI 用。密钥只存在本机配置里。

## 技术栈

| 层 | 选型 |
| --- | --- |
| 桌面壳 | Wails v2、WebView2 |
| 前端 | Vue 3、Pinia、Vite 5、TypeScript |
| UI | shadcn-vue（Reka UI）、Tailwind CSS 4、Lucide |
| 动效 | motion-v |
| 后端 | Go 1.22、pelletier/go-toml、json5 |

## 安全

- 配置默认只写本机磁盘，不上传任何服务。
- 云同步需要你自己提供对象存储凭证；对象内容使用口令派生的 AES-GCM 加密。
- 列表里的 API Key 会做掩码；完整值只在编辑表单中出现。
- 不要把 `config.json` 或导出的备份提交到 Git。

## 开发约定

- 前端包管理只用 pnpm。
- 不要用自制 Button / Input / Dialog 替代 `frontend/src/components/ui/` 里的 shadcn 组件。
- 窗口是无边框的，标题栏负责拖拽和窗口按钮。

## 更新日志

### 未发布

- OpenClaw Provider 移除，替换为 OpenCode：配置写入 `~/.config/opencode/opencode.json`，Skills 同步到 `~/.config/opencode/skills`，提示词支持 `~/.config/opencode/AGENTS.md`。

### v2.0.0

- 桌面壳改为顶栏文字导航 + 浅灰画布 + 白色圆角宫格，去掉左侧栏。
- 界面全面切到 shadcn-vue 与 Tailwind CSS 4，弹窗 / 下拉 / 输入 / 确认框统一走导入组件。
- 新增本机 API 路由（Anthropic ↔ OpenAI，含 Responses API）。
- 新增 S3 兼容云同步（含阿里云 OSS）。
- 新增 GitHub Release 检测与 Windows 应用内更新。
- OpenClaw、监控轮换、Skills 预设库与用量统计一并纳入同一工作台。

### v1.0.6

- OpenClaw Provider 全链路。
- Skills 同步到 `~/.openclaw/skills`。
- MCP 编辑弹窗二次打开表单为空的修复。

更早的记录见 Git 标签 `v1.0.0` … `v1.0.5`。

## 贡献

Issue 和 Pull Request 开在 [nsmao-com/claude-code-env-change](https://github.com/nsmao-com/claude-code-env-change)。改 UI 时请保持现有 shadcn 组件与顶栏导航结构。

## 许可证

[MIT](./LICENSE)
