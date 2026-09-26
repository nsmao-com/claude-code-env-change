<p align="center">
  <img src="build/appicon.png?v=2.7.3" width="72" height="72" alt="AI ENV icon" />
</p>

<h1 align="center">AI ENV</h1>

<p align="center">
  A local desktop workspace for Claude Code, Codex, Antigravity CLI (agy), OpenCode, and Grok.<br />
  Manage environments, MCP servers, skills, a local API router, uptime rotation, cloud backups, and installed CLIs in one window.
</p>

<p align="center">
  <a href="./README.md">中文</a> ·
  <a href="./README_EN.md"><strong>English</strong></a>
</p>

<p align="center"><a href="https://www.nsmao.com">Official website: www.nsmao.com</a></p>

<p align="center">
  <a href="https://github.com/nsmao-com/claude-code-env-change/releases"><img alt="Release" src="https://img.shields.io/github/v/release/nsmao-com/claude-code-env-change?style=flat-square" /></a>
  <a href="./LICENSE"><img alt="License" src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" /></a>
  <img alt="Go" src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square&logo=go&logoColor=white" />
  <img alt="Wails" src="https://img.shields.io/badge/Wails-v2-red?style=flat-square" />
  <img alt="Vue" src="https://img.shields.io/badge/Vue-3-42b883?style=flat-square&logo=vuedotjs&logoColor=white" />
  <img alt="Platform" src="https://img.shields.io/badge/platform-Windows-0078D4?style=flat-square&logo=windows&logoColor=white" />
</p>

<p align="center">
  <img src="portal.png" alt="AI ENV home" width="100%" />
</p>

## What it is

One native window for five CLI toolchains: environment variables, MCP servers, Skills, prompt files, usage stats, and local installs. Everything stays on disk unless you opt into S3-compatible backup. No third-party account is required to run the app.

The current version is the latest version listed in [GitHub Releases](https://github.com/nsmao-com/claude-code-env-change/releases).

## Features

| Module | What it does |
| --- | --- |
| Environments | Multiple profiles, per-tool filter, drag reorder, one-click apply, latency probe, drag-and-drop JSON import |
| MCP | stdio / HTTP servers, sync into Claude Code / Claude Desktop / Codex / Antigravity / OpenCode / Grok (remote servers reach Claude Desktop through an `npx mcp-remote` bridge, which needs Node.js) |
| Skills | Edit `SKILL.md`, import from online marketplaces or the bundled library, enable per platform |
| API router | Local gateway port and per-vendor switches; all five CLIs can convert between Anthropic Messages, Chat Completions, and Responses; each route can list backup upstreams that take over on rate limits, dead keys, or outages |
| Uptime | Periodic Base URL checks, optional key verification that catches expired keys or exhausted credit, and rotation groups |
| Cloud sync | S3 / Aliyun OSS / compatible endpoints, scrypt + AES-GCM encrypted objects |
| Prompts | Custom system prompts per CLI |
| Stats | Requests, tokens, cost estimate, model mix, activity heatmap |
| Settings | Language, theme, accent, outbound proxy |
| CLI | Detect local Claude / Codex / Antigravity / OpenCode / Grok; install/upgrade via pnpm, yarn, npm, official installer, or native update |
| Config folders | Open each CLI’s config directory and key files |
| Updates | GitHub Release check; Windows can download and replace in-app |

## 2.7.3 Tool filter fixes

- Fix the shared tool selector ignoring Claude Desktop in Prompts, Statistics, MCP and Skills.
- Show clear availability messages and a way back to supported tools for Claude Desktop prompts, skills and local usage statistics. Unsupported statistics filters no longer display all tools' data.
- Prevent stale statistics requests from overwriting a new selection, allow filter headers to wrap, and correct unsupported rotation-group defaults.

## 2.7.2 Workbench controls

- Use shared app components for workbench selects, inputs, textareas, checkboxes and buttons, with light/dark themes and keyboard support.
- Add a calendar popover with clear and range controls; session filters cover full days in the local time zone.
- Search model suggestions or enter a custom name. Number inputs provide step buttons, decimal precision and range limits.
- Use custom collapsible configuration diffs and in-app form validation feedback.

## 2.7.1 Fixes

- Use black icons throughout the app, system tray and installer.
- Restore Windows tray right-click handling, show a native menu while the panel initializes, and prevent double scaling on high-DPI displays.
- Fix template compilation for workbench session pagination and project editing.

## 2.7 Workbench

- **Sessions**: search local Claude Code, Codex and Gemini sessions with tool, project and date filters; paginate messages, export Markdown, open folders and resume through Claude/Codex CLI (Gemini is view/export only). AI ENV archives only hide entries in its list; native Codex archives remain read-only.
- **Models**: discover and test models, save context/output/compaction limits and input/output/cache prices. Codex supports model catalogs and official-login, API and keep-login modes; keep-login stores credentials under the selected provider.
- **Diagnostics and history**: inspect CLI availability, syntax, drift and optional network requests. MCP verifies stdio, Streamable HTTP and SSE using `initialize → initialized → tools/list`. Keep 100 pre-write snapshots, preview redacted changes and restore selected files with version checks.
- **Full Skills**: import folders, ZIPs and GitHub branches/tags/commits with attachments and executable flags. Review source revisions, per-file changes and platform edits; preserve local edits by default and recover previous central packages through history. Unsafe paths and package symlinks are rejected. Limits: 8 MB per file, 24 MB attachments, 1000 attachments; oversized imports fail explicitly. `.git` and `node_modules` are excluded.
- **Projects and prompts**: reusable prompt library and environment/model/MCP/Skills/prompt presets. Applying saves a snapshot first and switches the tool's global configuration, affecting other projects using that tool. Partial failures are reported and can be recovered through history.
- **Providers**: preview CC Switch JSON, share links (including base64 JSON / Codex TOML) and AI ENV exports. A universal provider creates linked environments across tools; apply the saved environments separately.
- **Gateway and costs**: priority, weighted and session routing; failure thresholds, cooldown and request-triggered recovery probes. View upstream health, actual token usage and streaming time to first token. Estimate USD costs with custom prices and multipliers, receive daily/monthly budget alerts and query OpenRouter / DeepSeek balances. Budgets do not block traffic.
- **Logs and cloud**: incremental Claude/Codex usage parsing and WebDAV support. ETag conditional uploads prevent overwrites. Restore previews redact credentials and allow file selection; local or remote edits invalidate the preview. Safe sync requires HEAD, ETag and conditional PUT support.

Gateway costs only include reported usage with a unique matching price profile. Ledgers rotate monthly and retain the current and two preceding months. CLI estimates use the environment activation timeline and remain separate to avoid double counting. Model test requests may incur provider charges.

New local data is stored in `~/.claude-env-switcher/`: `workbench.json` for profiles/presets/budgets, `history/` for original configuration snapshots (including credentials), and `gateway-usage.jsonl` for current-month usage without request bodies. Previews redact credentials; protect snapshots like original configuration files and use a cloud encryption passphrase.

Development verification uses an isolated home and local mock upstreams with real Wails/Vite dev and file writes. Regression checks: `go test ./...`, `go vet ./...`, and `cd frontend && pnpm exec vue-tsc --noEmit`.

## Install

Download and run the Windows installer from [Releases](https://github.com/nsmao-com/claude-code-env-change/releases):

```
claude-env-switcher-windows-amd64-installer.exe
```

The installer uses the application icon and lets you choose Start menu and desktop shortcuts separately. Upgrades and reinstalls reuse the previous installation directory, and the installer adds the [WebView2](https://developer.microsoft.com/microsoft-edge/webview2/) runtime when it is missing.

macOS and Linux can be built from source.

## Build from source

**Needs**

- Go 1.22+
- Node.js 18+ with **pnpm** only
- [Wails v2 CLI](https://wails.io)

```bash
git clone https://github.com/nsmao-com/claude-code-env-change.git
cd claude-code-env-change
go install github.com/wailsapp/wails/v2/cmd/wails@latest
cd frontend && pnpm install && cd ..
wails dev
```

Production build (NSIS is required for the Windows installer):

```bash
wails build -platform windows/amd64 -nsis -webview2 download
```

Output lands in `build/bin/`.

## Where data lives

```
~/.claude-env-switcher/
  config.json            environments
  mcp.json               MCP servers
  skills.json            skills index
  outbound-proxy.json    outbound proxy
```

Files written into each CLI:

| Tool | Path |
| --- | --- |
| Claude Code | `~/.claude/settings.json` |
| Codex | `~/.codex/config.toml`, `~/.codex/auth.json` (`CODEX_HOME`) |
| Claude Desktop (MCP) | `claude_desktop_config.json` in `%APPDATA%\\Claude` (Microsoft Store build: `%LOCALAPPDATA%\\Packages\\Claude_*\\LocalCache\\Roaming\\Claude`) or `~/Library/Application Support/Claude` |
| Antigravity CLI | `~/.gemini/antigravity-cli/settings.json`, `~/.gemini/config/mcp_config.json`; API key & endpoint are written to user environment variables (agy only reads env vars) |
| OpenCode | `~/.config/opencode/opencode.json` (`OPENCODE_CONFIG_DIR` / `OPENCODE_CONFIG`) |
| Grok | `~/.grok/config.toml` (`GROK_HOME`) |

A writable `config.json` next to the executable, left over from older builds, is still honored.

## Architecture

```
┌─────────────────────────────────────────────┐
│  Vue 3  ·  Pinia  ·  shadcn-vue  ·  Tailwind 4 │
│  motion-v  ·  Chart.js  ·  CodeMirror        │
└──────────────────────┬──────────────────────┘
                       │ Wails bindings
┌──────────────────────▼──────────────────────┐
│  Go  ·  Wails v2                             │
│  env / MCP / skills / local API gateway      │
│  uptime rotation / OSS sync / CLI upgrades   │
└─────────────────────────────────────────────┘
```

The local gateway translates Anthropic Messages, OpenAI Chat Completions, and OpenAI Responses. All five CLIs can select a target upstream format, so one upstream key can feed multiple CLIs. Keys never leave the machine unless you enable cloud sync.

## Stack

| Layer | Choice |
| --- | --- |
| Desktop | Wails v2, WebView2 |
| Frontend | Vue 3, Pinia, Vite 5, TypeScript |
| UI | shadcn-vue (Reka UI), Tailwind CSS 4, Lucide |
| Motion | motion-v |
| Backend | Go 1.22, pelletier/go-toml, json5 |

## Security

- Default path is local disk only.
- Cloud sync uses credentials you supply; objects are encrypted with AES-GCM under a scrypt-derived key. With a passphrase set, an unencrypted backup in the bucket is refused rather than restored.
- The local router gateway only accepts non-browser requests addressed to 127.0.0.1 / localhost, so web pages cannot spend your API keys through it.
- On macOS / Linux, local files that hold keys (`config.json`, `mcp.json`, `cloud.json`, …) are written with mode 600.
- API keys are masked in lists and shown in full only in the editor.
- Do not commit `config.json` or exported backups.

## Development notes

- Frontend installs go through pnpm, not npm or yarn.
- Import components from `frontend/src/components/ui/` instead of hand-rolling buttons, inputs, or dialogs.
- The window is frameless; the title bar owns dragging and window controls.

## Contributing

Issues and pull requests: [nsmao-com/claude-code-env-change](https://github.com/nsmao-com/claude-code-env-change). Keep the shadcn component set and the top navigation when touching UI.

Release notes live on [Releases](https://github.com/nsmao-com/claude-code-env-change/releases), not in this file.

## License

[MIT](./LICENSE)
