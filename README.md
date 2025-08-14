# Anthropic 环境变量管理器

![应用界面预览](portal.png)

一个用Go语言编写的现代化桌面应用，帮助您轻松管理和切换Anthropic API的环境变量配置。本工具采用iOS 18风格设计，使用Wails框架构建，提供了美观直观的用户界面。

## 功能特性

- 🎨 **iOS 18风格设计** - 现代化的用户界面，无渐变纯色设计
- 🖥️ **桌面应用** - 使用Wails框架构建的原生桌面应用
- 🔄 **多配置管理** - 支持创建、编辑、删除多个环境变量配置
- ⚡ **一键切换** - 快速切换不同的Anthropic API环境配置
- 📱 **响应式设计** - 支持深色模式，适配不同屏幕尺寸
- 💾 **本地存储** - 配置自动保存到本地JSON文件
- 🔒 **安全可靠** - 本地存储敏感信息，无网络传输
- 🚀 **跨平台支持** - Windows、macOS、Linux全平台支持
- 🎯 **实时反馈** - 环境变量状态实时显示和更新

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

# 或使用提供的批处理文件
.\run.bat
```

## 使用说明

### 首次运行
程序首次运行时会自动创建配置文件，并打开桌面GUI界面。

### 主要功能

#### 📊 当前环境变量显示
- 实时显示当前系统中的 `ANTHROPIC_BASE_URL` 和 `ANTHROPIC_AUTH_TOKEN`
- 支持手动刷新环境变量状态
- 清晰的状态指示（已设置/未设置）

#### 📋 配置管理
- **查看配置列表** - 显示所有已保存的环境变量配置
- **新增配置** - 通过表单创建新的环境变量配置
- **编辑配置** - 修改现有配置的名称和变量值
- **删除配置** - 移除不需要的配置
- **应用配置** - 一键将选定配置应用到系统环境变量

#### 🎨 用户体验
- **iOS 18风格界面** - 现代化的卡片式设计
- **深色模式支持** - 自动适配系统主题
- **响应式布局** - 适配不同屏幕尺寸
- **实时状态反馈** - 操作结果即时显示

### 添加环境配置

在Web界面中填写表单：
- 配置名称（如：Production、Development）
- API端点URL
- API密钥
- 其他环境变量

### 切换环境配置

在环境配置列表中点击对应配置的"切换"按钮即可立即切换。

### 删除环境配置

在环境配置列表中点击对应配置的"删除"按钮即可移除。

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
        "ANTHROPIC_AUTH_TOKEN": "your-dev-token"
      }
    },
    {
      "name": "Production",
      "description": "生产环境配置",
      "variables": {
        "ANTHROPIC_BASE_URL": "https://api.anthropic.com",
        "ANTHROPIC_AUTH_TOKEN": "your-prod-token"
      }
    }
  ]
}
```

## 支持的环境变量

本工具主要管理以下Anthropic API相关的环境变量：

- **ANTHROPIC_BASE_URL**: Anthropic API的基础URL
- **ANTHROPIC_AUTH_TOKEN**: Anthropic API的认证令牌

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

#### libpng iCCP 警告
如果在运行时看到以下警告信息：
```
libpng warning: iCCP: known incorrect sRGB profile
```

这是由于PNG图片文件中包含了不正确的sRGB颜色配置文件导致的。这个警告不影响程序功能，但如果需要修复，可以使用以下方法：

1. **使用ImageMagick修复**（推荐）：
   ```bash
   # 安装ImageMagick后执行
   magick mogrify -strip *.png
   ```

2. **使用在线工具**：
   - 将PNG文件上传到在线PNG优化工具
   - 重新下载优化后的文件

3. **使用图像编辑软件**：
   - 在Photoshop、GIMP等软件中重新保存PNG文件
   - 保存时选择不包含颜色配置文件

## 技术栈

- **后端语言**: Go 1.21+
- **前端技术**: HTML, CSS, JavaScript
- **桌面框架**: Wails v2
- **配置**: JSON格式
- **平台**: 跨平台支持（Windows、macOS、Linux）
- **依赖**: Wails框架, WebView2(Windows)/WebKit(Linux)/WKWebView(macOS)

## 许可证

MIT License