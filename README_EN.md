<p align="center">
  <img src="build/appicon.png" width="72" height="72" alt="AI ENV" />
</p>

<h1 align="center">AI ENV</h1>

<p align="center">
  A local desktop workspace for Claude Code, Codex, Gemini CLI, OpenCode, and Grok.<br />
  Manage environments, MCP servers, skills, a local API router, uptime rotation, cloud backups, and installed CLIs in one window.
</p>

<p align="center">
  <a href="./README.md">中文</a> ·
  <a href="./README_EN.md"><strong>English</strong></a>
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
  <img src="portal.png" alt="AI ENV screenshot" width="100%" />
</p>

## What it is

One native window for five CLI toolchains: environment variables, MCP servers, Skills, prompt files, usage stats, and local installs. Everything stays on disk unless you opt into S3-compatible backup. No third-party account is required to run the app.

Current release: **v2.2.0**. Release notes and binaries live on [GitHub Releases](https://github.com/nsmao-com/claude-code-env-change/releases).

## Features

| Module | What it does |
| --- | --- |
| Environments | Multiple profiles, per-tool filter, drag reorder, one-click apply, latency probe, drag-and-drop JSON import |
| MCP | stdio / HTTP servers, sync into Claude / Codex / Gemini / OpenCode / Grok |
| Skills | Edit `SKILL.md`, import from the bundled library, enable per platform |
| API router | Local protocol gateway (Anthropic Messages, Chat Completions, Responses) |
| Uptime | Periodic Base URL checks and rotation groups |
| Cloud sync | S3 / Aliyun OSS / compatible endpoints, AES-GCM encrypted objects |
| Prompts | Custom system prompts per CLI |
| Stats | Requests, tokens, cost estimate, model mix, activity heatmap |
| Settings | Language, theme, accent, outbound proxy |
| CLI | Detect local Claude / Codex / Gemini / OpenCode / Grok and upgrade via pnpm, npm, or native update |
| Config folders | Open each CLI’s config directory and key files |
| Updates | GitHub Release check; Windows can download and replace in-app |

## Install

Download the Windows build from [Releases](https://github.com/nsmao-com/claude-code-env-change/releases) and unzip:

```
claude-env-switcher-windows-amd64.zip
```

[WebView2](https://developer.microsoft.com/microsoft-edge/webview2/) is required. Windows 11 already ships it.

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

Production build:

```bash
wails build
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
| Codex | `~/.codex/config.toml`, `~/.codex/auth.json` |
| Gemini CLI | `~/.gemini/.env`, `~/.gemini/settings.json` |
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

The local gateway translates Anthropic Messages, OpenAI Chat Completions, and Codex Responses so one upstream key can feed multiple CLIs. Keys never leave the machine unless you enable cloud sync.

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
- Cloud sync uses credentials you supply; objects are encrypted with a passphrase-derived AES-GCM key.
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
