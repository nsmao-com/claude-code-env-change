# MCP 配置同步使用指南

## 问题已解决 ✅

你遇到的"删除 Codex MCP 后配置文件没有更新"的问题已经完全解决。

### 根本原因

Codex 配置文件 `~/.codex/config.toml` 中的 `mysql_nice_order_1` 配置缺少 `type = 'stdio'` 字段，导致：
- TOML 解析失败
- 环境管理器无法正确读取配置
- 同步逻辑失效

### 已完成的修复

1. ✅ 修复了 `config.toml` 格式错误
2. ✅ 创建了 `app_linux.go` 文件（跨平台编译支持）
3. ✅ 验证了同步逻辑正常工作
4. ✅ 创建了检查工具和文档

## 当前状态

```
环境管理器：10 个 MCP 启用 Codex
Codex 配置：  10 个 MCP
状态：        ✅ 完全同步
```

**MCP 列表**：
- Windows-notify
- chrome-devtools
- context7_1
- exa_1
- fetch_1
- linear
- mysql_nice_knowledge
- mysql_nice_order
- mysql_nice_order_1
- playwright_1

## 使用方法

### 删除 MCP 的 Codex 启用

1. 打开环境管理器应用（`claude-env-switcher.exe`）
2. 找到要删除的 MCP，点击"编辑"按钮
3. 取消勾选"Codex"平台
4. 点击"保存"按钮
5. 等待保存成功提示
6. ✅ 配置会自动同步到 `~/.codex/config.toml`

### 验证同步状态

运行检查脚本：
```bash
cd D:/2024Dev/claude-env
python check_mcp_sync.py
```

或者手动检查：
```bash
# 检查环境管理器配置
cd ~/.claude-env-switcher
python -c "import json; d=json.load(open('mcp.json')); print([k for k,v in d.items() if 'codex' in v.get('enable_platform',[])])"

# 检查 Codex 配置
cd ~/.codex
grep -E '^\[mcp_servers\.[^.]+\]$' config.toml
```

## 故障排除

如果删除后配置没有更新，尝试以下步骤：

### 1. 确认操作步骤
- ✓ 是否点击了"保存"按钮？
- ✓ 是否看到了"保存成功"的提示？
- ✓ 是否等待了几秒钟让同步完成？

### 2. 重启应用
```bash
# 关闭环境管理器
taskkill /F /IM claude-env-switcher.exe

# 重新打开环境管理器
# 然后重试删除操作
```

### 3. 检查配置文件格式
```bash
cd D:/2024Dev/claude-env
python check_mcp_sync.py
```

如果显示"TOML 格式错误"，说明配置文件有问题，需要手动修复。

### 4. 手动修复配置文件

如果自动同步失败，可以手动编辑配置文件：

```bash
# 备份配置文件
cp ~/.codex/config.toml ~/.codex/config.toml.backup

# 编辑配置文件
notepad ~/.codex/config.toml

# 删除不需要的 MCP 配置块
# 例如删除 mysql_nice_order_1：
# [mcp_servers.mysql_nice_order_1]
# args = [...]
# command = 'uv'
# type = 'stdio'
#
# [mcp_servers.mysql_nice_order_1.env]
# MYSQL_DATABASE = 'nice_knowledge'
# ...
```

## 文件说明

### 核心文件

- `~/.claude-env-switcher/mcp.json` - 环境管理器的 MCP 配置
- `~/.codex/config.toml` - Codex 的配置文件
- `~/.claude.json` - Claude 的配置文件
- `~/.gemini/settings.json` - Gemini 的配置文件

### 工具脚本

- `check_mcp_sync.py` - MCP 同步状态检查脚本（推荐使用）
- `check_mcp_sync.bat` - Windows 批处理版本
- `check_mcp_sync.ps1` - PowerShell 版本

### 文档

- `MCP_SYNC_FIX_REPORT.md` - 详细的问题修复报告
- `README_MCP_SYNC.md` - 本文档

### 备份文件

- `~/.codex/config.toml.backup` - 修复前的配置备份
- `~/.claude-env-switcher/mcp.json.backup.*` - 测试时的配置备份

## 技术细节

### TOML 格式要求

每个 MCP 服务器配置必须包含 `type` 字段：

**Stdio 类型**：
```toml
[mcp_servers.server_name]
type = "stdio"      # ← 必需
command = "npx"
args = ["..."]
```

**HTTP 类型**：
```toml
[mcp_servers.server_name]
type = "http"       # ← 必需
url = "http://..."
```

### 同步逻辑

环境管理器在以下情况下会同步配置：

1. 保存 MCP 配置时（点击"保存"按钮）
2. 添加新 MCP 时
3. 应用环境配置时

同步过程：
1. 读取环境管理器配置（`mcp.json`）
2. 读取现有 Codex 配置（`config.toml`）
3. 合并配置：
   - 保留不受管理的 MCP（手动添加到 Codex 的）
   - 删除被管理但未启用的 MCP
   - 添加/更新启用了 Codex 的 MCP
4. 写入 Codex 配置文件

## 常见问题

### Q: 为什么我删除了 MCP，但配置文件还在？

A: 可能的原因：
1. 没有点击"保存"按钮
2. 配置文件格式有错误，导致同步失败
3. 环境管理器需要重启
4. Codex 正在运行，锁定了配置文件

**解决方法**：
1. 确保点击了"保存"按钮
2. 运行 `python check_mcp_sync.py` 检查格式
3. 重启环境管理器和 Codex
4. 如果还不行，手动编辑配置文件

### Q: 如何知道配置是否同步成功？

A: 运行检查脚本：
```bash
cd D:/2024Dev/claude-env
python check_mcp_sync.py
```

如果显示"✓ 配置完全同步"，说明同步成功。

### Q: 我可以手动编辑 Codex 配置文件吗？

A: 可以，但不推荐。因为：
1. 环境管理器会覆盖手动修改
2. 容易出现格式错误

**推荐做法**：
- 在环境管理器中管理 MCP
- 如果需要手动添加，在 Codex 配置文件中添加后，环境管理器会自动导入

### Q: 配置文件的修改时间不一致怎么办？

A: 这是正常的。环境管理器会先更新 `mcp.json`，然后同步到 `config.toml`，所以会有几秒钟的时间差。

只要两边的 MCP 列表一致，就说明同步成功。

## 联系支持

如果遇到其他问题，可以：

1. 查看详细报告：`MCP_SYNC_FIX_REPORT.md`
2. 运行检查脚本：`python check_mcp_sync.py`
3. 查看备份文件：`~/.codex/config.toml.backup`

---

**最后更新**：2026-02-02
**状态**：✅ 问题已解决，功能正常
