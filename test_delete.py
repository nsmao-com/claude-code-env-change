#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""测试 MCP 服务器删除功能"""

import json
import os
from pathlib import Path

def test_delete():
    # 配置文件路径
    cache_file = Path.home() / ".claude-env-switcher" / "mcp.json"
    codex_file = Path.home() / ".codex" / "config.toml"

    print("=" * 60)
    print("MCP 服务器删除功能测试")
    print("=" * 60)
    print()

    # 1. 读取缓存文件
    print("[1] 读取缓存文件...")
    if not cache_file.exists():
        print(f"   ❌ 缓存文件不存在: {cache_file}")
        return

    with open(cache_file, 'r', encoding='utf-8') as f:
        cache_data = json.load(f)

    print(f"   ✓ 缓存文件中有 {len(cache_data)} 个服务器")
    print()

    # 2. 列出所有服务器
    print("[2] 当前服务器列表:")
    for i, name in enumerate(sorted(cache_data.keys()), 1):
        platforms = cache_data[name].get('enable_platform', [])
        print(f"   {i:2d}. {name:30s} - {', '.join(platforms)}")
    print()

    # 3. 选择要删除的服务器
    print("[3] 请选择要删除的服务器编号（输入 0 取消）:")
    try:
        choice = int(input("   > "))
        if choice == 0:
            print("   取消删除")
            return

        server_names = sorted(cache_data.keys())
        if choice < 1 or choice > len(server_names):
            print(f"   ❌ 无效的选择: {choice}")
            return

        server_to_delete = server_names[choice - 1]
        print(f"   选择删除: {server_to_delete}")
        print()

    except (ValueError, KeyboardInterrupt):
        print("   取消删除")
        return

    # 4. 确认删除
    print(f"[4] 确认删除 '{server_to_delete}'? (y/n)")
    confirm = input("   > ").strip().lower()
    if confirm != 'y':
        print("   取消删除")
        return
    print()

    # 5. 从缓存中删除
    print("[5] 从缓存文件中删除...")
    if server_to_delete in cache_data:
        del cache_data[server_to_delete]
        with open(cache_file, 'w', encoding='utf-8') as f:
            json.dump(cache_data, f, indent=2, ensure_ascii=False)
        print(f"   ✓ 已从缓存中删除")
    else:
        print(f"   ❌ 服务器不存在于缓存中")
    print()

    # 6. 从 Codex 配置中删除
    print("[6] 从 Codex 配置中删除...")
    if codex_file.exists():
        with open(codex_file, 'r', encoding='utf-8') as f:
            codex_content = f.read()

        if f"[mcp_servers.{server_to_delete}]" in codex_content:
            # 简单的删除逻辑（仅用于测试）
            lines = codex_content.split('\n')
            new_lines = []
            skip = False

            for line in lines:
                if line.strip() == f"[mcp_servers.{server_to_delete}]":
                    skip = True
                    continue
                elif line.strip().startswith('[mcp_servers.') and skip:
                    skip = False

                if not skip:
                    new_lines.append(line)

            with open(codex_file, 'w', encoding='utf-8') as f:
                f.write('\n'.join(new_lines))

            print(f"   ✓ 已从 Codex 配置中删除")
        else:
            print(f"   ℹ 服务器不存在于 Codex 配置中")
    else:
        print(f"   ℹ Codex 配置文件不存在")
    print()

    # 7. 验证删除结果
    print("[7] 验证删除结果...")

    # 检查缓存
    with open(cache_file, 'r', encoding='utf-8') as f:
        cache_data = json.load(f)

    if server_to_delete in cache_data:
        print(f"   ❌ 缓存中仍然存在: {server_to_delete}")
    else:
        print(f"   ✓ 缓存中已删除: {server_to_delete}")

    # 检查 Codex
    if codex_file.exists():
        with open(codex_file, 'r', encoding='utf-8') as f:
            codex_content = f.read()

        if f"[mcp_servers.{server_to_delete}]" in codex_content:
            print(f"   ❌ Codex 配置中仍然存在: {server_to_delete}")
        else:
            print(f"   ✓ Codex 配置中已删除: {server_to_delete}")

    print()
    print("=" * 60)
    print("测试完成")
    print("=" * 60)

if __name__ == '__main__':
    test_delete()
