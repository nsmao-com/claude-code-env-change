#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""MCP 配置同步状态检查脚本"""

import json
import os
import re
from pathlib import Path

def main():
    print("=" * 60)
    print("  MCP 配置同步状态检查")
    print("=" * 60)
    print()

    home = Path.home()

    # 1. 检查环境管理器配置
    print("[1] 环境管理器配置")
    print("-" * 60)
    mcp_json_path = home / ".claude-env-switcher" / "mcp.json"

    codex_mcps = []
    if mcp_json_path.exists():
        with open(mcp_json_path, 'r', encoding='utf-8') as f:
            config = json.load(f)

        for name, server in config.items():
            if 'codex' in server.get('enable_platform', []):
                codex_mcps.append(name)

        print(f"启用 Codex 的 MCP: {len(codex_mcps)} 个")
        for name in sorted(codex_mcps):
            print(f"  - {name}")
    else:
        print("✗ 配置文件不存在")
    print()

    # 2. 检查 Codex 配置文件
    print("[2] Codex 配置文件")
    print("-" * 60)
    codex_config_path = home / ".codex" / "config.toml"

    codex_servers = []
    if codex_config_path.exists():
        with open(codex_config_path, 'r', encoding='utf-8') as f:
            content = f.read()

        # 提取 MCP 服务器名称
        pattern = r'^\[mcp_servers\.([^.]+)\]$'
        for line in content.split('\n'):
            match = re.match(pattern, line)
            if match:
                codex_servers.append(match.group(1))

        print(f"Codex 配置中的 MCP: {len(codex_servers)} 个")
        for name in sorted(codex_servers):
            print(f"  - {name}")
    else:
        print("✗ 配置文件不存在")
    print()

    # 3. 配置文件修改时间
    print("[3] 配置文件修改时间")
    print("-" * 60)
    if mcp_json_path.exists():
        mtime = mcp_json_path.stat().st_mtime
        from datetime import datetime
        dt = datetime.fromtimestamp(mtime)
        print(f"mcp.json:    {dt.strftime('%Y-%m-%d %H:%M:%S')}")

    if codex_config_path.exists():
        mtime = codex_config_path.stat().st_mtime
        from datetime import datetime
        dt = datetime.fromtimestamp(mtime)
        print(f"config.toml: {dt.strftime('%Y-%m-%d %H:%M:%S')}")
    print()

    # 4. 对比结果
    print("[4] 同步状态")
    print("-" * 60)

    codex_set = set(codex_mcps)
    servers_set = set(codex_servers)

    if len(codex_set) == len(servers_set):
        print(f"✓ 数量一致 ({len(codex_set)} 个)")

        if codex_set == servers_set:
            print("✓ 配置完全同步")
        else:
            print("✗ 配置不一致")
            print("\n差异:")

            only_in_manager = codex_set - servers_set
            if only_in_manager:
                print("  环境管理器有，Codex 没有:")
                for name in sorted(only_in_manager):
                    print(f"    - {name}")

            only_in_codex = servers_set - codex_set
            if only_in_codex:
                print("  Codex 有，环境管理器没有:")
                for name in sorted(only_in_codex):
                    print(f"    + {name}")
    else:
        print("✗ 数量不一致")
        print(f"  环境管理器: {len(codex_set)} 个")
        print(f"  Codex 配置: {len(servers_set)} 个")

    print()
    print("=" * 60)

    # 5. 检查 TOML 格式
    print("\n[5] TOML 格式验证")
    print("-" * 60)
    try:
        import tomli
        with open(codex_config_path, 'rb') as f:
            data = tomli.load(f)
        print("✓ TOML 格式正确")
        print(f"✓ MCP 服务器数量: {len(data.get('mcp_servers', {}))}")
    except ImportError:
        try:
            import toml
            with open(codex_config_path, 'r') as f:
                data = toml.load(f)
            print("✓ TOML 格式正确")
            print(f"✓ MCP 服务器数量: {len(data.get('mcp_servers', {}))}")
        except ImportError:
            print("⚠ 未安装 tomli 或 toml 库，跳过 TOML 验证")
    except Exception as e:
        print(f"✗ TOML 格式错误: {e}")

    print()

if __name__ == '__main__':
    main()
    input("\n按 Enter 键退出...")
