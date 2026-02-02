@echo off
echo ========================================
echo MCP 服务器清理验证脚本
echo ========================================
echo.

echo [1] 检查 Codex 配置文件中的 mysql_nice_order_1...
findstr /C:"mysql_nice_order_1" "%USERPROFILE%\.codex\config.toml" >nul 2>&1
if %errorlevel% equ 0 (
    echo    ❌ 仍然存在于 Codex 配置中
) else (
    echo    ✅ 不存在于 Codex 配置中 ^(正确^)
)
echo.

echo [2] 检查缓存文件中的 mysql_nice_order_1...
findstr /C:"mysql_nice_order_1" "%USERPROFILE%\.claude-env-switcher\mcp.json" >nul 2>&1
if %errorlevel% equ 0 (
    echo    ❌ 仍然存在于缓存文件中 ^(需要刷新^)
) else (
    echo    ✅ 不存在于缓存文件中 ^(已清理^)
)
echo.

echo [3] 统计缓存文件中的 MCP 服务器数量...
for /f %%i in ('findstr /C:"\"type\":" "%USERPROFILE%\.claude-env-switcher\mcp.json" ^| find /c /v ""') do set count=%%i
echo    当前缓存中有 %count% 个 MCP 服务器
echo.

echo ========================================
echo 验证完成
echo ========================================
echo.
echo 如果 mysql_nice_order_1 仍在缓存中，请：
echo 1. 打开应用
echo 2. 进入 MCP 服务器管理
echo 3. 点击"刷新"按钮
echo 4. 再次运行此脚本验证
echo.
pause
