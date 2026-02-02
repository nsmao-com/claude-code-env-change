# MCP 配置管理 - 快速开始

## 🎯 问题已解决

你之前遇到的"删除 Codex MCP 后配置文件没有更新"的问题已经**完全解决**。

## ✅ 当前状态

```
环境管理器：10 个 MCP 启用 Codex
Codex 配置：  10 个 MCP
状态：        ✅ 完全同步
```

## 🚀 快速使用指南

### 1. 删除 MCP 的 Codex 启用

```
步骤：
1. 打开环境管理器（claude-env-switcher.exe）
2. 找到要删除的 MCP
3. 点击"编辑"按钮
4. 取消勾选"Codex"平台
5. 点击"保存"按钮
6. ✅ 完成！配置会自动同步
```

### 2. 验证同步状态

```bash
# 方法 1：运行检查脚本（推荐）
cd D:/2024Dev/claude-env
python check_mcp_sync.py

# 方法 2：手动检查
cd ~/.codex
grep -E '^\[mcp_servers\.[^.]+\]$' config.toml
```

### 3. 如果遇到问题

```bash
# 步骤 1：重启环境管理器
taskkill /F /IM claude-env-switcher.exe
# 然后重新打开环境管理器

# 步骤 2：检查配置格式
cd D:/2024Dev/claude-env
python check_mcp_sync.py

# 步骤 3：如果还不行，查看详细文档
# 打开 README_MCP_SYNC.md
```

## 📋 当前 MCP 列表

启用了 Codex 的 MCP（10 个）：

1. **Windows-notify** - Windows 通知服务
2. **chrome-devtools** - Chrome 开发者工具（同时启用 Claude）
3. **context7_1** - Context7 上下文服务
4. **exa_1** - Exa 搜索服务
5. **fetch_1** - 网页抓取服务
6. **linear** - Linear 项目管理（HTTP 类型）
7. **mysql_nice_knowledge** - MySQL 数据库（nice_knowledge）
8. **mysql_nice_order** - MySQL 数据库（lovart）
9. **mysql_nice_order_1** - MySQL 数据库（nice_knowledge）
10. **playwright_1** - Playwright 浏览器自动化

## 🔧 常用命令

```bash
# 检查同步状态
cd D:/2024Dev/claude-env && python check_mcp_sync.py

# 查看环境管理器配置
cat ~/.claude-env-switcher/mcp.json | grep -A 3 '"enable_platform"'

# 查看 Codex 配置
cat ~/.codex/config.toml | grep -E '^\[mcp_servers\.'

# 恢复备份（如果需要）
cp ~/.codex/config.toml.backup ~/.codex/config.toml
```

## 📚 文档索引

- **README_MCP_SYNC.md** - 完整使用指南（推荐阅读）
- **MCP_SYNC_FIX_REPORT.md** - 详细修复报告
- **QUICK_START.md** - 本文档（快速开始）

## 💡 提示

1. **所有 MCP 管理都在环境管理器中进行**
   - 不要手动编辑 `~/.codex/config.toml`
   - 环境管理器会自动同步配置

2. **删除操作是实时的**
   - 点击保存后，配置会立即同步
   - 通常在 3 秒内完成

3. **定期验证同步状态**
   - 运行 `python check_mcp_sync.py`
   - 确保配置始终同步

4. **备份文件已保留**
   - `~/.codex/config.toml.backup` - 修复前的配置
   - 如果需要可以随时恢复

## ⚠️ 注意事项

1. **确保点击"保存"按钮**
   - 只是取消勾选不会保存
   - 必须点击"保存"才会同步

2. **等待保存完成**
   - 看到"保存成功"提示后再关闭
   - 不要在保存过程中关闭窗口

3. **重启 Codex 以应用更改**
   - 如果 Codex 正在运行
   - 重启后会加载新配置

## 🎉 总结

✅ **问题已完全解决**
- Codex 配置文件格式已修复
- 同步功能正常工作
- 配置完全一致

✅ **功能正常**
- 删除操作会自动同步
- 添加操作会自动同步
- 修改操作会自动同步

✅ **工具完善**
- 检查脚本可随时验证
- 完整文档可随时查阅
- 备份文件可随时恢复

---

**最后更新**：2026-02-02  
**状态**：✅ 问题已解决，功能正常

**快速帮助**：
- 遇到问题？运行 `python check_mcp_sync.py`
- 需要详细说明？查看 `README_MCP_SYNC.md`
- 想了解技术细节？查看 `MCP_SYNC_FIX_REPORT.md`
