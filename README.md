# Claude Code 环境管理器

![应用界面预览](portal.png)

一个用Go语言编写的现代化桌面应用，专为Claude Code用户设计，帮助您轻松管理和切换Anthropic API的环境变量配置。本工具采用现代毛玻璃(Glassmorphism)设计风格，使用Wails框架构建，提供简洁优雅的用户界面。

## 功能特性

### 🎨 现代化UI设计
- **毛玻璃效果** - 采用Glassmorphism设计风格，透明模糊背景
- **简洁配色** - 统一的灰蓝色调，去除花哨配色
- **流畅动画** - 平滑的过渡效果和交互反馈
- **响应式布局** - 适配不同屏幕尺寸，优化视觉层次

### ⚡ 核心功能
- 🖥️ **原生桌面应用** - 基于Wails v2框架构建
- 🔄 **多配置管理** - 创建、编辑、删除多个环境变量配置
- ⚡ **一键应用** - 快速切换不同的Anthropic API环境配置
- 🎯 **实时监控** - 环境变量状态实时显示和刷新
- 💾 **本地存储** - 配置安全保存到本地JSON文件

### 🔒 安全特性
- **敏感信息保护** - API密钥和令牌在界面中部分隐藏
- **本地存储** - 所有配置本地保存，无网络传输
- **系统集成** - 直接写入系统环境变量，支持持久化

### 🌟 用户体验
- **顶部消息提示** - 优雅的Toast消息系统替代弹窗
- **按钮尺寸统一** - 所有操作按钮保持一致的视觉效果
- **直观操作** - 清晰的图标和操作提示
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
- **实时显示** - 当前系统中的 `ANTHROPIC_BASE_URL`、`ANTHROPIC_AUTH_TOKEN`、`ANTHROPIC_MODEL` 和 `ANTHROPIC_API_KEY`
- **状态指示** - 清晰显示已设置/未设置状态，支持快速识别
- **一键刷新** - 手动刷新环境变量状态，确保信息实时更新
- **单项清除** - 支持清除单个环境变量，精确控制

#### 📋 配置管理中心
- **配置列表** - 以卡片形式展示所有已保存的环境变量配置
- **新增配置** - 通过现代化表单界面创建新的环境变量配置
- **编辑配置** - 修改现有配置的名称和变量值
- **删除配置** - 安全删除不需要的配置（带确认提示）
- **一键应用** - 将选定配置快速应用到系统环境变量

#### 🎨 现代化界面
- **毛玻璃设计** - 采用透明模糊背景的现代设计语言
- **统一配色** - 简洁的灰蓝色调，避免视觉干扰
- **智能布局** - 左侧操作面板，右侧主要内容，布局合理
- **优雅提示** - 顶部Toast消息提示，替代传统弹窗

#### 🔒 安全保护
- **敏感信息隐藏** - API密钥和令牌在列表中自动部分遮蔽
- **安全删除确认** - 重要操作需要用户确认，防止误操作
- **本地加密存储** - 所有配置信息安全保存在本地

### 📝 操作指南

#### 添加新配置
1. 点击右上角的 **"新增配置"** 按钮
2. 在弹出的表单中填写：
   - **配置名称** - 给配置起个容易识别的名字（如：Production、Development）
   - **ANTHROPIC_BASE_URL** - API服务器地址
   - **ANTHROPIC_AUTH_TOKEN** - 认证令牌
   - **ANTHROPIC_MODEL** - 模型名称（如：claude-3-5-sonnet-20241022）
   - **ANTHROPIC_API_KEY** - API密钥
3. 点击 **"保存配置"** 完成添加

#### 应用配置
1. 在配置列表中找到目标配置
2. 点击 **"应用"** 按钮
3. 系统会自动将配置应用到环境变量
4. 顶部会显示成功提示消息

#### 编辑配置
1. 点击配置卡片右上角的 **"编辑"** 按钮
2. 在表单中修改需要更新的信息
3. 点击 **"保存配置"** 完成更新

#### 删除配置
1. 点击配置卡片右上角的 **"删除"** 按钮
2. 在确认对话框中点击 **"确定"**
3. 配置将被永久删除

#### 管理环境变量
- **刷新状态** - 点击左侧面板的 **"刷新环境变量"** 按钮
- **清除单个变量** - 在监控面板中点击对应变量旁的 ❌ 按钮
- **清除所有变量** - 点击左侧面板的 **"清除所有变量"** 按钮

## 配置文件

程序会在当前目录下创建 `config.json` 文件来存储环境变量配置：

```json
{
  "current_env": "Development",
  "environments": [
    {
      "name": "Development",
      "description": "开发环境配置",
      "variables": {
        "ANTHROPIC_BASE_URL": "https://api.anthropic.com",
        "ANTHROPIC_AUTH_TOKEN": "your-dev-token",
        "ANTHROPIC_MODEL": "claude-3-5-sonnet-20241022"
      }
    },
    {
      "name": "Production",
      "description": "生产环境配置",
      "variables": {
        "ANTHROPIC_BASE_URL": "https://api.anthropic.com",
        "ANTHROPIC_AUTH_TOKEN": "your-prod-token",
        "ANTHROPIC_MODEL": "claude-3-5-sonnet-20241022"
      }
    }
  ]
}
```

## 支持的环境变量

本工具主要管理以下Anthropic API相关的环境变量：

- **ANTHROPIC_BASE_URL**: Anthropic API的基础URL
- **ANTHROPIC_AUTH_TOKEN**: Anthropic API的认证令牌  
- **ANTHROPIC_MODEL**: Anthropic API使用的模型名称（如：claude-3-5-sonnet-20241022）
- **ANTHROPIC_API_KEY**: Anthropic API的密钥

### 使用场景
- 在不同的API提供商之间切换（如官方API、代理API等）
- 开发环境和生产环境的快速切换
- 团队协作中的配置标准化
- API密钥的安全管理

## 注意事项

- 🔒 **安全提醒**: 请妥善保管您的API密钥，不要将包含密钥的配置文件提交到版本控制系统
- 📁 **配置文件**: 首次使用时程序会自动创建默认配置文件
- 🔄 **环境变量**: 环境变量的设置是永久性的，会写入系统注册表（Windows）
- 💾 **备份建议**: 建议定期备份 `config.json` 配置文件
- 🖥️ **系统要求**: Windows系统需要WebView2运行时来运行Wails应用
- ⚡ **即时生效**: 环境变量设置后立即生效，无需重启应用

### 常见问题

#### Q: 为什么环境变量没有立即生效？
A: 环境变量设置后，可能需要重启相关应用程序才能生效。点击"刷新环境变量"按钮可以检查当前状态。

#### Q: 如何备份我的配置？
A: 直接复制项目目录下的 `config.json` 文件即可。建议定期备份此文件以防数据丢失。

#### Q: 是否支持自定义环境变量？
A: 目前版本专注于Claude Code相关的环境变量管理。如需管理其他环境变量，可以手动编辑 `config.json` 文件。

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

### 设计特色
- **响应式设计** - 适配不同屏幕尺寸
- **现代动画** - CSS3过渡和变换效果  
- **无障碍支持** - 符合现代Web标准
- **跨平台兼容** - Windows、macOS、Linux全支持

## 📝 更新日志

### v1.1.0 (最新版本)
**🎨 UI 全面重构**
- 采用现代毛玻璃(Glassmorphism)设计风格
- 统一灰蓝色调，去除花哨配色  
- 优化按钮尺寸，确保视觉一致性
- 改进卡片布局和交互反馈

**✨ 功能优化**
- 新增顶部Toast消息提示系统
- 改进敏感信息显示（API密钥部分隐藏）
- 优化环境变量监控界面
- 增强表单输入体验

**🔧 技术改进**
- 引入Inter字体提升可读性
- 优化CSS动画和过渡效果
- 改进响应式布局
- 代码结构优化和清理

### v1.0.0 (初始版本)
- 基础环境变量管理功能
- 多配置支持
- 跨平台桌面应用
- JSON配置文件存储

## 📄 许可证

MIT License

---

**Claude Code 环境管理器** - 让环境变量管理更简单、更优雅 ✨