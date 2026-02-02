@echo off
chcp 65001 >nul
echo ========================================
echo MCP 服务器删除同步测试
echo ========================================
echo.

set SERVER_NAME=%1
if "%SERVER_NAME%"=="" (
    echo 用法: test_delete_sync.bat [服务器名称]
    echo 例如: test_delete_sync.bat mysql_nice_order
    echo.
    pause
    exit /b 1
)

echo [1] 检查缓存文件中的 %SERVER_NAME%...
findstr /C:"%SERVER_NAME%" "%USERPROFILE%\.claude-env-switcher\mcp.json" >nul 2>&1
if %errorlevel% equ 0 (
    echo    ✓ 存在于缓存文件中
) else (
    echo    ✗ 不存在于缓存文件中
)
echo.

echo [2] 检查 Codex 配置中的 %SERVER_NAME%...
findstr /C:"%SERVER_NAME%" "%USERPROFILE%\.codex\config.toml" >nul 2>&1
if %errorlevel% equ 0 (
    echo    ✓ 存在于 Codex 配置中
) else (
    echo    ✗ 不存在于 Codex 配置中
)
echo.

echo [3] 检查 Claude 配置中的 %SERVER_NAME%...
findstr /C:"%SERVER_NAME%" "%USERPROFILE%\.claude.json" >nul 2>&1
if %errorlevel% equ 0 (
    echo    ✓ 存在于 Claude 配置中
) else (
    echo    ✗ 不存在于 Claude 配置中
)
echo.

echo ========================================
echo 测试完成
echo ========================================
echo.
echo 如果服务器已删除，所有检查应该显示 "✗ 不存在"
echo.
pause
