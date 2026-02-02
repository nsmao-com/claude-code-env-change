# Claude Code 环境管理器

![应用界面预览](portal.png)

一个用Go语言编写的现代化桌面应用，支持多种 AI CLI 工具（Claude Code、Codex、Gemini CLI）的环境变量配置管理。本工具采用现代 Bento Grid 设计风格，使用Wails框架构建，提供简洁优雅的用户界面。

## 功能特性

### 🎨 现代化UI设计
- **Bento Grid 布局** - 采用现代卡片式网格布局，视觉层次清晰
- **Dark Mode 支持** - 内置深色/浅色主题切换
- **流畅动画** - 平滑的过渡效果和交互反馈
- **响应式设计** - 适配不同屏幕尺寸，自动调整布局

### ⚡ 核心功能
- 🖥️ **原生桌面应用** - 基于Wails v2框架构建
- 🔄 **多 Provider 支持** - 支持 Claude、Codex、Gemini CLI 三种工具
- 📦 **多配置管理** - 创建、编辑、删除多个环境变量配置
- 🎯 **拖拽排序** - 自由拖拽配置卡片调整顺序
- 🔍 **智能筛选** - 按 Provider 筛选和搜索配置
- ⚡ **一键应用** - 快速切换不同的 API 环境配置
- 🌐 **测速功能** - 实时测试 API 端点延迟
- 📝 **自定义模板** - Codex 和 Gemini 支持自定义配置文件模板
- 💾 **本地存储** - 配置安全保存到本地JSON文件

### 🔒 安全特性
- **敏感信息保护** - API密钥和令牌在界面中部分隐藏
- **本地存储** - 所有配置本地保存，无网络传输
- **系统集成** - 直接写入系统环境变量，支持持久化

### 🌟 用户体验
- **Toast 消息系统** - 优雅的消息提示替代传统弹窗
- **实时状态监控** - 环境变量状态实时显示和刷新
- **直观操作** - 清晰的图标和操作提示
- **配置保序更新** - 编辑配置后保持原有顺序
- **🚀 跨平台支持** - Windows、macOS、Linux全平台支持

## 安装和运行

### 前置要求
- Go 1.21 或更高版本
- Wails CLI v2
- WebView2 运行时（Windows）

### 快速开始

#### 1. 安装Wails CLI
```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

#### 2. 克隆项目
```bash
git clone <repository-url>
cd claude_change
```

#### 3. 安装依赖
```bash
go mod tidy
```

#### 4. 运行开发模式
```bash
wails dev
```

#### 5. 构建生产版本
```bash
wails build
```

### 直接运行
如果您已经有编译好的可执行文件，可以直接运行：
```bash
# Windows
.\build\bin\claude-env-switcher.exe
```

## 使用说明

### 首次运行
程序首次运行时会自动创建配置文件，并打开桌面GUI界面。

### 主要功能

#### 📊 环境变量监控面板
- **实时显示** - 当前系统中的 Claude Code 环境变量（`ANTHROPIC_BASE_URL`、`ANTHROPIC_AUTH_TOKEN`、`ANTHROPIC_MODEL`、`ANTHROPIC_API_KEY`）
- **状态指示** - 清晰显示已设置/未设置状态，支持快速识别
- **一键刷新** - 手动刷新环境变量状态，确保信息实时更新
- **单项清除** - 支持清除单个环境变量，精确控制

#### 📋 配置管理中心
- **多 Provider 支持** - 支持 Claude、Codex、Gemini CLI 三种工具的配置管理
- **配置列表** - 以卡片网格形式展示所有已保存的配置
- **拖拽排序** - 通过拖拽卡片自由调整配置顺序
- **智能筛选** - 按 Provider 类型筛选配置（All/Claude/Codex/Gemini）
- **实时搜索** - 通过配置名称快速搜索定位
- **新增配置** - 支持创建不同 Provider 的环境变量配置
- **编辑配置** - 修改配置内容，自动保持原有位置
- **删除配置** - 安全删除不需要的配置（带确认提示）
- **一键应用** - 将选定配置快速应用到对应的工具
- **测速功能** - 实时测试 API 端点的网络延迟

#### 🎨 现代化界面
- **Bento Grid 布局** - 采用卡片式网格布局，视觉现代化
- **Dark/Light 主题** - 支持深色和浅色主题切换
- **Provider 标识** - 不同 Provider 使用不同的颜色标签区分
- **智能布局** - 左侧操作面板，右侧内容区域，布局合理
- **优雅提示** - Toast 消息提示，替代传统弹窗

#### 🔒 安全保护
- **敏感信息隐藏** - API密钥和令牌在列表中自动部分遮蔽
- **安全删除确认** - 重要操作需要用户确认，防止误操作
- **本地存储** - 所有配置信息安全保存在本地

### 📝 操作指南

#### 添加新配置
1. 点击左侧面板的 **"新建配置"** 按钮
2. 选择 Provider 类型（Claude / Codex / Gemini CLI）
3. 根据选择的 Provider 填写相应字段：

   **Claude 配置**：
   - 配置名称
   - Base URL（可选）
   - Auth Token（可选）
   - Model（可选）
   - API Key（可选）

   **Codex 配置**：
   - 配置名称
   - Base URL
   - API Key
   - Model
   - 可选：自定义 config.toml 和 auth.json 模板

   **Gemini CLI 配置**：
   - 配置名称
   - Base URL
   - API Key
   - Model
   - 可选：自定义 .env 和 settings.json 模板

4. 点击 **"保存配置"** 完成添加

#### 应用配置
1. 在配置列表中找到目标配置
2. 点击配置卡片（或点击配置卡片上的按钮）
3. 系统会自动将配置应用：
   - **Claude**：设置系统环境变量
   - **Codex**：生成 `~/.codex/config.toml` 和 `~/.codex/auth.json`
   - **Gemini**：生成 `~/.gemini/.env` 和 `~/.gemini/settings.json`
4. 顶部会显示成功提示消息

#### 测速功能
1. 将鼠标悬停在配置卡片上
2. 点击 **测速图标**（速度计图标）
3. 系统会实时测试该配置的 Base URL 延迟
4. 结果会显示在按钮上，并以颜色标识：
   - 绿色：< 300ms
   - 黄色：300-1000ms
   - 红色：> 1000ms

#### 拖拽排序
1. 按住配置卡片
2. 拖动到目标位置
3. 释放鼠标完成排序
4. 排序会自动保存到配置文件

#### 筛选和搜索
- **按 Provider 筛选**：点击配置库上方的标签（All / Claude / Codex / Gemini）
- **搜索配置**：在搜索框中输入配置名称进行实时搜索

#### 编辑配置
1. 点击配置卡片右上角的 **"编辑"** 图标（铅笔）
2. 在表单中修改需要更新的信息
3. 点击 **"保存配置"** 完成更新
4. 配置会保持在原有位置

#### 删除配置
1. 点击配置卡片右上角的 **"删除"** 图标（垃圾桶）
2. 在确认对话框中点击 **"确定"**
3. 配置将被永久删除

#### 管理环境变量
- **刷新状态** - 点击左侧面板的 **"刷新状态"** 按钮
- **清除单个变量** - 在监控面板中点击对应变量旁的 ❌ 按钮
- **清除所有变量** - 点击左侧面板的 **"清除所有变量"** 按钮

#### 切换主题
- 点击左侧面板底部的 **Dark Mode** 开关切换深色/浅色主题

## 配置文件

程序会把主配置写入用户目录：`~/.claude-env-switcher/config.json`（与 `mcp.json` / `skills.json` 同目录）。  
兼容旧版本：若启动目录下已存在 `config.json` 且可写，会继续使用该文件。macOS 若应用安装在 `/Applications`，应用目录通常不可写，建议使用默认的用户目录配置文件。

```json
{
  "current_env": "Development",
  "current_env_claude": "Development",
  "current_env_codex": "Codex Production",
  "current_env_gemini": "Gemini Dev",
  "environments": [
    {
      "name": "Development",
      "description": "Claude 开发环境",
      "provider": "claude",
      "variables": {
        "ANTHROPIC_BASE_URL": "https://api.anthropic.com",
        "ANTHROPIC_AUTH_TOKEN": "your-dev-token",
        "ANTHROPIC_MODEL": "claude-3-5-sonnet-20241022",
        "ANTHROPIC_API_KEY": "your-api-key"
      }
    },
    {
      "name": "Codex Production",
      "description": "Codex 生产环境",
      "provider": "codex",
      "variables": {
        "base_url": "https://your-codex-api.com",
        "OPENAI_API_KEY": "your-openai-key",
        "model": "gpt-5.1-codex"
      },
      "templates": {
        "config.toml": "model_provider = \"duckcoding\"\nmodel = \"{{model}}\"\n...",
        "auth.json": "{\n  \"OPENAI_API_KEY\": \"{{OPENAI_API_KEY}}\"\n}"
      }
    },
    {
      "name": "Gemini Dev",
      "description": "Gemini CLI 开发环境",
      "provider": "gemini",
      "variables": {
        "GOOGLE_GEMINI_BASE_URL": "https://generativelanguage.googleapis.com",
        "GEMINI_API_KEY": "your-gemini-key",
        "GEMINI_MODEL": "gemini-3-pro-preview"
      },
      "templates": {
        ".env": "GOOGLE_GEMINI_BASE_URL={{GOOGLE_GEMINI_BASE_URL}}\n...",
        "settings.json": "{\n  \"ide\": {\n    \"enabled\": true\n  }\n}"
      }
    }
  ]
}
```

## 支持的配置类型

### Claude Code 配置
管理 Claude Code 相关的环境变量：
- **ANTHROPIC_BASE_URL**: API 基础 URL
- **ANTHROPIC_AUTH_TOKEN**: 认证令牌
- **ANTHROPIC_MODEL**: 模型名称（如：claude-3-5-sonnet-20241022）
- **ANTHROPIC_API_KEY**: API 密钥

### Codex 配置
生成 Codex CLI 所需的配置文件（`~/.codex/`）：
- **base_url**: Codex API 基础 URL
- **OPENAI_API_KEY**: OpenAI API 密钥
- **model**: 模型名称
- **自定义模板**: 支持自定义 config.toml 和 auth.json 模板

### Gemini CLI 配置
生成 Gemini CLI 所需的配置文件（`~/.gemini/`）：
- **GOOGLE_GEMINI_BASE_URL**: Gemini API 基础 URL
- **GEMINI_API_KEY**: Gemini API 密钥
- **GEMINI_MODEL**: 模型名称
- **自定义模板**: 支持自定义 .env 和 settings.json 模板

### 使用场景
- 在不同的 AI CLI 工具之间快速切换
- 在不同的 API 提供商之间切换（如官方 API、代理 API 等）
- 开发环境和生产环境的快速切换
- 团队协作中的配置标准化
- API 密钥的安全管理

## 注意事项

- 🔒 **安全提醒**: 请妥善保管您的API密钥，不要将包含密钥的配置文件提交到版本控制系统
- 📁 **配置文件**: 首次使用时程序会自动创建默认配置文件
- 🔄 **环境变量**: 环境变量的设置是永久性的，会写入系统注册表（Windows）
- 💾 **备份建议**: 建议定期备份 `~/.claude-env-switcher/config.json` 配置文件
- 🖥️ **系统要求**: Windows系统需要WebView2运行时来运行Wails应用
- ⚡ **即时生效**: 环境变量设置后立即生效，无需重启应用

### 常见问题

#### Q: 为什么环境变量没有立即生效？
A: 环境变量设置后，可能需要重启相关应用程序才能生效。点击"刷新环境变量"按钮可以检查当前状态。

#### Q: 如何备份我的配置？
A: 默认复制 `~/.claude-env-switcher/config.json` 即可。若你之前使用旧版本/便携模式，也可能是启动目录下的 `config.json`。

#### Q: 是否支持自定义环境变量？
A: 支持！您可以：
1. 对于 Claude：直接添加任意环境变量
2. 对于 Codex/Gemini：使用自定义模板功能自定义配置文件内容

#### Q: 拖拽排序后配置丢失怎么办？
A: 拖拽排序会自动保存到配置文件（默认 `~/.claude-env-switcher/config.json`）。如果出现问题，请检查文件权限或从备份恢复。

#### Q: 不同 Provider 的配置可以同时激活吗？
A: 可以！每个 Provider 独立管理，可以同时激活 Claude、Codex、Gemini 的不同配置。

#### Q: Windows系统提示需要WebView2怎么办？
A: 请访问 Microsoft 官网下载并安装 WebView2 运行时，这是 Wails 应用在 Windows 上运行的必要组件。

## 🛠️ 技术栈

### 核心技术
- **后端语言**: Go 1.21+
- **前端技术**: HTML5, CSS3, JavaScript (ES6+)
- **桌面框架**: Wails v2
- **UI设计**: Glassmorphism (毛玻璃效果)
- **字体**: Inter Font Family

### 依赖组件
- **运行时**: WebView2 (Windows) / WebKit (Linux) / WKWebView (macOS)
- **数据存储**: JSON本地文件
- **图标库**: Font Awesome 6.4.0
- **CSS框架**: Tailwind CSS (CDN)
- **拖拽库**: Sortable.js 1.15.0

### 设计特色
- **响应式设计** - 适配不同屏幕尺寸
- **现代动画** - CSS3过渡和变换效果  
- **无障碍支持** - 符合现代Web标准
- **跨平台兼容** - Windows、macOS、Linux全支持

## 📝 更新日志

### v1.0.1 (2025-12-22)
**🔧 Bug 修复**
- 修复 Codex 日志解析结构，正确读取 `event_msg.payload.info.total_token_usage`
- 修复 Codex 日志路径，支持 `~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl` 结构
- 新增 `gpt-5.1-codex-max` 模型定价支持

**📊 统计功能**
- 新增 Codex 平台统计支持
- 新增 Gemini 平台统计支持
- 统计弹窗支持按平台筛选（全部/Claude/Gemini/Codex）

### v1.0.0 (初始版本)
**🚀 核心功能**
- 多 Provider 支持（Claude/Codex/Gemini CLI）
- 配置拖拽排序
- 按 Provider 筛选和搜索配置
- API 端点测速功能
- 自定义配置文件模板（Codex/Gemini）
- 基础环境变量管理
- 跨平台桌面应用
- JSON 配置文件存储

## 📄 许可证

MIT License

---

**Claude Code 环境管理器** - 让环境变量管理更简单、更优雅 ✨
