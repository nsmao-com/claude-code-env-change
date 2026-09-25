package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

func validateCloudBundle(b cloudBundle) error {
	for name, raw := range b.Files {
		if !json.Valid(raw) || string(raw) == "null" {
			return fmt.Errorf("%s 不是有效配置对象", name)
		}
		switch name {
		case "config.json":
			var c Config
			if e := json.Unmarshal(raw, &c); e != nil {
				return e
			}
			for _, env := range c.Environments {
				if strings.TrimSpace(env.Name) == "" {
					return fmt.Errorf("备份环境缺少名称")
				}
				if _, ok := knownProvider(env.Provider); !ok && env.Provider != "claude_desktop" {
					return fmt.Errorf("备份包含未知工具")
				}
			}
		case "mcp.json":
			var c map[string]rawMCPServer
			if e := json.Unmarshal(raw, &c); e != nil {
				return e
			}
			for n, s := range c {
				if strings.TrimSpace(n) == "" {
					return fmt.Errorf("MCP 名称为空")
				}
				if s.Type == "http" || s.Type == "sse" {
					if e := validEndpoint(s.URL); e != nil {
						return e
					}
				} else if strings.TrimSpace(s.Command) == "" {
					return fmt.Errorf("MCP 命令为空")
				}
			}
		case "skills.json":
			var c map[string]rawSkill
			if e := json.Unmarshal(raw, &c); e != nil {
				return e
			}
			for n, s := range c {
				if !isSafeSkillDirName(n) {
					return fmt.Errorf("不安全的技能目录")
				}
				if e := validateSkillFiles(s.Files); e != nil {
					return e
				}
			}
		case "router.json":
			var c RouterConfig
			if e := json.Unmarshal(raw, &c); e != nil {
				return e
			}
			for _, r := range c.Routes {
				if e := validateRoutePolicy(r); e != nil {
					return e
				}
				if !routeNamePattern.MatchString(r.Name) {
					return fmt.Errorf("无效的路由名称")
				}
				if e := validEndpoint(r.BaseURL); e != nil {
					return e
				}
			}
		case "workbench.json":
			var c WorkbenchConfig
			if e := json.Unmarshal(raw, &c); e != nil {
				return e
			}
		}
	}
	return nil
}
