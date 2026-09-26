<p align="center">
  <img src="build/appicon.png?v=2.7.3" width="72" height="72" alt="AI ENV 图标" />
</p>

<h1 align="center">AI ENV</h1>

<p align="center">
  面向 Claude Code、Claude Desktop、Codex、Antigravity CLI（agy）、OpenCode、Grok 的本地桌面工作台。<br />
  一处管理环境配置、MCP、Skills、本地 API 路由、监控轮换、云端备份和本机 CLI。
</p>

<p align="center">
  <a href="./README.md"><strong>中文</strong></a> ·
  <a href="./README_EN.md">English</a>
</p>

<p align="center"><a href="https://www.nsmao.com">官网：www.nsmao.com</a></p>

<p align="center">
  <a href="https://github.com/nsmao-com/claude-code-env-change/releases"><img alt="Release" src="https://img.shields.io/github/v/release/nsmao-com/claude-code-env-change?style=flat-square" /></a>
  <a href="./LICENSE"><img alt="License" src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" /></a>
  <img alt="Go" src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square&logo=go&logoColor=white" />
  <img alt="Wails" src="https://img.shields.io/badge/Wails-v2-red?style=flat-square" />
  <img alt="Vue" src="https://img.shields.io/badge/Vue-3-42b883?style=flat-square&logo=vuedotjs&logoColor=white" />
  <img alt="Platform" src="https://img.shields.io/badge/platform-Windows-0078D4?style=flat-square&logo=windows&logoColor=white" />
</p>

<p align="center">
  <img src="portal.png" alt="AI ENV 首页" width="100%" />
</p>

## 这是什么

把 Claude Code、Claude Desktop、Codex、Antigravity CLI、OpenCode、Grok 的环境变量、MCP 服务器、Skills、提示词、用量统计和本机安装收进同一个原生窗口。配置写在本机，不经过第三方账号；换电脑时可以用 S3 兼容对象存储加密备份。

当前版本以 [GitHub Releases](https://github.com/nsmao-com/claude-code-env-change/releases) 的最新版本为准。

## 功能

| 模块 | 说明 |
| --- | --- |
| 环境 | 多配置、按平台筛选、拖拽排序、一键写入对应 CLI、延迟测速、JSON 拖拽导入 |
| MCP | 管理 stdio / HTTP 服务器，同步到 Claude Code / Claude Desktop / Codex / Antigravity / OpenCode / Grok（Claude Desktop 的远程服务器通过 `npx mcp-remote` 桥接，需本机有 Node.js） |
| Skills | 完整技能包、在线市场、附件管理、来源更新与本地修改保护，按平台启用 |
| API 路由 | 本机网关端口与按厂商开关；各平台可在 Anthropic Messages、Chat Completions、Responses 之间转换；每条路由可配备用上游，限流、Key 失效或宕机时自动切换 |
| 监控 | 定时探测 Base URL，可选「用 Key 验证」发现 Key 失效 / 余额不足，按轮换组自动切配置 |
| 云同步 | S3 / 阿里云 OSS / WebDAV，scrypt + AES-GCM 加密、ETag 冲突保护、预览与选择恢复 |
| 提示词 | 编辑各平台自定义系统提示词 |
| 统计 | 请求量、Token、花费估算、模型分布、活动热力图 |
| 设置 | 语言、主题、强调色、出站代理 |
| CLI | 检测本机 Claude Code / Codex / Antigravity / OpenCode / Grok，按 pnpm、yarn、npm、官方安装器或原生方式安装升级 |
| 配置目录 | 打开各家 CLI 的本机配置目录和关键文件 |
| 更新 | 检测 GitHub Release，Windows 可在应用内下载并替换 |

## 2.7.3 工具页签修复

- 修复公共工具筛选器遗漏 Claude Desktop，提示词、统计、MCP、Skills 页签现在可以正常选择。
- 为尚未接入的 Claude Desktop 提示词、Skills 及本地用量统计显示明确说明和切换入口；统计页不再把未支持的平台回退成全部数据。
- 修复快速切换统计平台时旧请求覆盖新选择、窄窗口筛选栏挤压标题，以及轮换组的无效默认平台。

## 2.7.2 工作台控件

- 工作台下拉、输入框、文本域、复选框和按钮统一使用应用组件，适配浅色、深色主题与键盘操作。
- 日期筛选改为日历弹层，支持清除和范围约束，按本地时区的完整自然日检索。
- 模型候选支持搜索与自定义名称；数字输入提供加减按钮，保留小数精度和上下限。
- 配置差异使用自定义展开面板，表单校验通过应用内提示反馈。

## 2.7.1 修复

- 应用、托盘与安装图标统一为黑色。
- 修复 Windows 托盘右键不响应；面板未就绪时立即显示原生菜单，并修复高 DPI 下的重复缩放。
- 修复工作台会话分页和项目编辑按钮的模板编译问题。

## 2.7 工作台

- **会话**：搜索 Claude Code、Codex、Gemini 的本地会话，按工具、项目、日期筛选并分页；阅读消息、导出 Markdown、打开目录、通过 Claude/Codex CLI 继续会话（Gemini 仅查看和导出）。AI ENV 归档只影响列表，不移动原始日志；Codex 原生归档只读。
- **模型**：从环境发现可用模型，发送最小测试请求；保存上下文、最大输出、自动压缩阈值以及输入/输出/缓存价格。Codex 支持模型目录及官方登录、API、保留官方登录三种模式；后者把 API 凭证放在指定 provider 下。
- **诊断与历史**：检查 CLI、配置语法、配置漂移和可选网络请求；MCP 通过 `initialize → initialized → tools/list` 验证 stdio、Streamable HTTP 与 SSE。保留最近 100 份配置写入前快照，支持脱敏差异和按文件恢复；提交恢复时会重新核对本地版本。
- **完整 Skills**：导入本地目录、ZIP 或指定 GitHub 分支/标签/commit 下的完整技能，保留附件和可执行脚本标记。显示来源 commit、逐文件更新差异及本地修改；默认保留本地修改，旧中央存储可从配置历史恢复。禁止越界路径和包内符号链接；单文件 8 MB、附件合计 24 MB、最多 1000 个附件，超限会明确拒绝。`.git` 和 `node_modules` 不导入。
- **项目与提示词**：维护可复用提示词库，把环境、模型、MCP、Skills、提示词组合成项目套装。应用前保存配置快照；套装切换的是工具的全局配置，会影响使用同一工具的其他项目。多步应用失败时会报告已写入部分，可通过历史恢复。
- **供应商**：预览导入 CC Switch JSON、分享链接（含 base64 JSON / Codex TOML 配置）及 AI ENV 配置；通用供应商可生成多个工具的关联环境。保存后需在环境页应用。
- **网关策略与费用**：支持优先级、加权轮询、按会话分流，以及失败阈值、冷却和请求触发的恢复探测；显示健康状态、上游 Token 和流式首 Token 延迟。按上游实际返回用量、自定义美元价格与环境倍率估算费用，日/月预算仅提醒，不拦截请求；支持 OpenRouter / DeepSeek 余额接口。
- **日志与云端**：Claude/Codex 用量日志增量解析；新增 WebDAV。上传用 ETag 条件写防覆盖，恢复先预览脱敏差异并按文件选择，本地或远端变更会让预览失效。安全同步要求存储支持 HEAD、ETag 和条件 PUT。

费用统计只计入网关收到有效 usage 的请求，未上报或无法唯一匹配价格的请求不计费用；网关账本按月轮转，保留当月及前两个月。CLI 日志费用依据激活时间线归属环境，两种口径分开显示，不重复累加。模型连通性请求可能产生供应商费用。

新增本地数据均位于 `~/.claude-env-switcher/`：`workbench.json` 保存模型/套装/预算，`history/` 保存原始配置快照（含凭证），`gateway-usage.jsonl` 保存当前月网关用量而不保存请求正文。差异界面会隐藏凭证，但本地快照仍需像原始配置一样保护；云备份建议设置加密口令。

开发验证使用独立用户目录和本地模拟上游，覆盖真实 Wails/Vite dev 及配置落盘；自动回归可运行 `go test ./...`、`go vet ./...` 和 `cd frontend && pnpm exec vue-tsc --noEmit`。

## 安装

从 [Releases](https://github.com/nsmao-com/claude-code-env-change/releases) 下载 Windows 安装包并双击安装：

```
claude-env-switcher-windows-amd64-installer.exe
```

安装器带有应用图标，可分别选择是否创建开始菜单和桌面快捷方式；升级或重装时会自动沿用上次的安装目录，并在系统缺少 [WebView2](https://developer.microsoft.com/microsoft-edge/webview2/) 时自动安装运行环境。

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

生产构建（Windows 安装包需要 NSIS）：

```bash
wails build -platform windows/amd64 -nsis -webview2 download
```

产物在 `build/bin/`。

## 数据放在哪

主配置目录：

```
~/.claude-env-switcher/
  config.json            环境配置
  mcp.json               MCP 服务器
  skills.json            Skills 索引
  outbound-proxy.json    出站代理
```

应用写入的 CLI 文件（按平台）：

| 平台 | 路径 |
| --- | --- |
| Claude Code | `~/.claude/settings.json` |
| Claude Desktop | 新版 3P：Windows `%LOCALAPPDATA%\\Claude-3p\\configLibrary\\<id>.json`（当前配置见同目录 `_meta.json`）；旧版与 MCP：`%APPDATA%\\Claude\\claude_desktop_config.json`（微软商店版在 `%LOCALAPPDATA%\\Packages\\Claude_*\\LocalCache\\Roaming\\Claude\\`）；macOS：`~/Library/Application Support/Claude/claude_desktop_config.json` |
| Codex | `~/.codex/config.toml`、`~/.codex/auth.json`（可用 `CODEX_HOME` 覆盖） |
| Antigravity CLI | `~/.gemini/antigravity-cli/settings.json`、`~/.gemini/config/mcp_config.json`；密钥/端点写入用户环境变量（agy 只认环境变量） |
| OpenCode | `~/.config/opencode/opencode.json`（可用 `OPENCODE_CONFIG_DIR` / `OPENCODE_CONFIG` 覆盖） |
| Grok | `~/.grok/config.toml`（可用 `GROK_HOME` 覆盖） |

旧版本若在启动目录留下了可写的 `config.json`，会继续使用该文件。

## 架构

```
┌─────────────────────────────────────────────┐
│  Vue 3  ·  Pinia  ·  shadcn-vue  ·  Tailwind 4 │
│  motion-v  ·  Chart.js  ·  CodeMirror        │
└──────────────────────┬──────────────────────┘
                       │ Wails bindings
┌──────────────────────▼──────────────────────┐
│  Go  ·  Wails v2                             │
│  环境 / MCP / Skills / 路由网关               │
│  监控轮换 / OSS 云同步 / CLI 检测升级          │
└─────────────────────────────────────────────┘
```

本地路由网关把 Anthropic Messages、OpenAI Chat Completions 与 OpenAI Responses 互相转换，五家 CLI 均可选择目标上游格式，让同一份上游 Key 给多套 CLI 用。密钥只存在本机配置里。

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
- 云同步需要你自己提供对象存储凭证；对象内容用 scrypt 从口令派生密钥后 AES-GCM 加密。设置了口令时，拉到未加密的备份会拒绝恢复，防止存储桶被人篡改。
- 本机路由网关只接受 127.0.0.1 / localhost 的非浏览器请求，网页无法借它盗用你的 API Key。
- 存有 Key 的本地文件（`config.json`、`mcp.json`、`cloud.json` 等）在 macOS / Linux 上权限为 600，只对当前用户可读。
- 列表里的 API Key 会做掩码；完整值只在编辑表单中出现。
- 不要把 `config.json` 或导出的备份提交到 Git。

## 开发约定

- 前端包管理只用 pnpm。
- 不要用自制 Button / Input / Dialog 替代 `frontend/src/components/ui/` 里的 shadcn 组件。
- 窗口是无边框的，标题栏负责拖拽和窗口按钮。

## 贡献

Issue 和 Pull Request 开在 [nsmao-com/claude-code-env-change](https://github.com/nsmao-com/claude-code-env-change)。改 UI 时请保持现有 shadcn 组件与顶栏导航结构。

版本记录写在 [Releases](https://github.com/nsmao-com/claude-code-env-change/releases)，不在本文件维护。

## 许可证

[MIT](./LICENSE)
