package main

// 内置技能预设库：开箱即用的常用 AI 编码技能，可一键导入到各 CLI 平台。
// 内容中的代码围栏使用 ~~~（Markdown 标准支持的写法），避免 Go 原始字符串中的反引号冲突。

// SkillPreset 内置技能预设
type SkillPreset struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
}

// GetSkillPresets 返回内置技能预设库
func (ss *SkillService) GetSkillPresets() []SkillPreset {
	return builtinSkillPresets
}

var builtinSkillPresets = []SkillPreset{
	{
		Name:        "commit-helper",
		Description: "生成规范的 Conventional Commits 提交信息，自动归纳变更内容",
		Content: `---
name: commit-helper
description: 生成规范的 Conventional Commits 提交信息。当用户要求提交代码、写 commit message 时使用。
---

# Git 提交信息助手

## 工作流程

1. 运行 "git status" 和 "git diff --staged"（无暂存时用 "git diff"）了解变更
2. 若用户未暂存任何文件，先列出建议暂存的文件并询问确认
3. 依据变更内容生成提交信息

## 提交信息格式

遵循 Conventional Commits：

~~~
<type>(<scope>): <subject>

<body>

<footer>
~~~

- type：feat / fix / docs / style / refactor / perf / test / build / ci / chore
- scope：可选，表示影响的模块（如 auth、api、ui）
- subject：祈使句、不加句号、不超过 50 个字符
- body：说明"为什么"改，可省略
- footer：标注破坏性变更 "BREAKING CHANGE:" 或关闭的 issue

## 规则

- 一次提交只做一件事；混合了多个目的的变更建议拆分提交
- 不要提交敏感信息（密钥、token、密码、内网地址）
- 未经用户确认不要执行 "git commit"，先展示建议的提交信息
`,
	},
	{
		Name:        "code-reviewer",
		Description: "系统性代码审查：正确性、安全、性能、可维护性四维度走查",
		Content: `---
name: code-reviewer
description: 系统性代码审查。当用户要求 review 代码、检查代码质量时使用。
---

# 代码审查技能

## 审查维度（按优先级）

### 1. 正确性
- 边界条件：空值、空集合、越界、除零、并发竞争
- 错误处理：错误是否被吞掉、资源是否泄漏（defer close）
- 逻辑与需求是否一致

### 2. 安全
- 注入风险：SQL 拼接、命令拼接、XSS、路径穿越
- 敏感信息：日志/响应中是否泄漏密钥、token、个人数据
- 鉴权：接口是否校验权限、是否存在越权（IDOR）

### 3. 性能
- 循环内重复计算、N+1 查询、不必要的深拷贝
- 大集合处理是否应分页/流式
- 锁粒度与热点路径

### 4. 可维护性
- 命名是否达意、函数是否过长（>80 行需关注）
- 重复代码、魔法数字、硬编码路径
- 与项目现有风格是否一致

## 输出格式

按严重程度分级输出问题列表：

- **[P0] 阻断**：会导致故障或数据丢失，必须修复
- **[P1] 重要**：安全风险或明显缺陷，应尽快修复
- **[P2] 建议**：可维护性/性能改进
- **[P3] 风格**：命名、注释等小问题

每条问题给出：文件:行号、问题描述、修复建议（含示例代码）。最后给一句总体结论。
`,
	},
	{
		Name:        "test-writer",
		Description: "为指定代码生成边界覆盖完整的单元测试",
		Content: `---
name: test-writer
description: 生成单元测试。当用户要求写测试、补测试、提高覆盖率时使用。
---

# 单元测试生成技能

## 工作流程

1. 先通读被测代码，理解输入输出与副作用
2. 识别项目现有测试框架与风格（Go test / pytest / vitest / jest 等），保持一致
3. 设计用例清单（先想清楚再写）

## 用例设计清单

对每个公开函数至少考虑：

- 正常路径（典型输入）
- 边界：空、零值、单元素、极值
- 异常：非法输入、依赖报错时的行为
- 副作用：状态修改、调用次数（mock 断言）

## 原则

- 测试命名描述行为：如 "订单金额为负时抛出异常"
- 每个测试只验证一个行为，失败时能快速定位
- 不 mock 被测对象本身；依赖通过参数注入
- 优先用表驱动（table-driven）组织同类用例
- 不写依赖时间/随机数/网络的脆弱测试；必须依赖时注入 clock/seed
- 断言信息可读，包含期望值与实际值

## 输出

给出完整可运行的测试文件，并简要说明覆盖了哪些场景、还有哪些场景建议补充。
`,
	},
	{
		Name:        "changelog-generator",
		Description: "根据 git 历史或变更内容生成 Keep a Changelog 格式的变更日志",
		Content: `---
name: changelog-generator
description: 生成变更日志。当用户要求写 changelog、发布说明、release notes 时使用。
---

# 变更日志生成技能

## 工作流程

1. 确定版本范围："git log <上一版本tag>..HEAD"
2. 逐条分析提交，按用户影响重新归类（不以 commit type 机械照搬）
3. 输出结构化日志

## 格式（Keep a Changelog）

~~~markdown
## [版本号] - 日期

### Added / 新增
- ...

### Changed / 变更
- ...

### Fixed / 修复
- ...

### Removed / 移除
- ...
~~~

## 原则

- 站在使用者视角描述"什么变了"，而不是"改了哪些代码"
- 破坏性变更置顶并给出迁移方法
- 合并含义重复的提交；忽略纯重构、CI 调整等对用户不可见的变更
- 中英项目保持语言一致；不确定版本号时提示用户确认
`,
	},
	{
		Name:        "sql-optimizer",
		Description: "分析 SQL 与表结构，给出索引、改写与分页优化建议",
		Content: `---
name: sql-optimizer
description: SQL 优化建议。当用户要求优化 SQL、排查慢查询时使用。
---

# SQL 优化技能

## 分析步骤

1. 拿到 SQL 与执行计划（EXPLAIN / EXPLAIN ANALYZE）；没有计划时先说明需假设的前提
2. 定位主要开销：全表扫描、filesort、临时表、rows 估算过大
3. 给出优化方案并解释预期收益

## 常见优化手段

- 索引：为 WHERE / JOIN / ORDER BY 列建复合索引，遵循最左前缀；区分度低的列不单独建索引
- 改写：避免 "SELECT *"、前置百分号 LIKE、对索引列使用函数或隐式类型转换
- 分页：大偏移量改用游标（keyset pagination）："WHERE id > ? LIMIT n"
- 聚合：先缩小数据集再 JOIN/聚合；必要时用覆盖索引避免回表
- 批量：大批量 UPDATE/DELETE 分批执行，避免长事务与主从延迟

## 输出

- 指出当前主要瓶颈及证据（计划中的关键行）
- 给出改写后的 SQL（标注差异）
- 给出建议的 DDL（CREATE INDEX ...）并提醒在低峰期执行、注意写入放大
`,
	},
	{
		Name:        "regex-helper",
		Description: "编写、解释与调试正则表达式，注重性能与可读性",
		Content: `---
name: regex-helper
description: 正则表达式编写与解释。当用户要求写正则、解释正则、优化正则时使用。
---

# 正则表达式技能

## 编写流程

1. 明确目标语言（各语言 flavor 有差异：Go RE2 不支持回溯引用、JS 支持 lookahead）
2. 列出必须匹配与必须不匹配的样例（正例 + 反例）
3. 编写正则并逐个样例验证

## 原则

- 优先具体字符类（[0-9] 优于 .），避免灾难性回溯
- 用非贪婪量词减少回溯；嵌套量词（如 (a+)+）要特别警惕
- 复杂正则拆分并加注释（Python re.VERBOSE、JS 命名分组）；或改用多个简单正则/普通字符串处理
- 捕获用命名分组，便于维护

## 解释输出

给出正则时同步给出：结构分解表（片段 → 含义）、至少 2 组匹配/不匹配样例、复杂度提示（是否回溯密集）。
`,
	},
	{
		Name:        "refactor-advisor",
		Description: "评估代码坏味道并给出小步安全的重构方案",
		Content: `---
name: refactor-advisor
description: 重构建议。当用户要求重构、改善代码结构、消除坏味道时使用。
---

# 重构建议技能

## 工作流程

1. 先确认是否有测试保护；没有测试的重构先建议补关键路径测试
2. 识别坏味道并按收益/风险排序
3. 给出小步重构方案（每步可独立验证），不一次性大改

## 常见坏味道 → 手法

- 过长函数 → 提取函数（按意图命名，而非细节）
- 重复代码 → 提取公共函数/模板；相似但不完全相同时参数化差异
- 过深嵌套 → 卫语句提前返回、提取条件
- 巨型条件 → 策略表/映射表替代 if-else 链
- 全局可变状态 → 依赖注入、显式传参
- 数据泥团 → 提取结构体/对象

## 原则

- 行为不变是底线：每步重构后运行测试
- 一次 PR 只做一种重构类型，不混合功能变更
- 命名即文档：重构后让代码自解释，删掉多余注释
- 输出时给出"重构前 → 重构后"对照与验证方式
`,
	},
	{
		Name:        "doc-writer",
		Description: "撰写清晰的 README、API 文档与模块说明",
		Content: `---
name: doc-writer
description: 技术文档撰写。当用户要求写 README、API 文档、使用说明时使用。
---

# 技术文档撰写技能

## 先想读者

写之前明确文档给谁看、读者要完成什么任务。按任务组织内容，而不是按代码结构。

## README 结构

1. 一句话说明项目是什么、解决什么问题
2. 快速开始：安装 → 最小可运行示例 → 常见用法
3. 配置说明：表格列出环境变量/配置项（名称、类型、默认值、说明）
4. 进阶主题与 FAQ
5. 参与贡献 / 许可证

## API 文档结构

- 接口：方法、路径、鉴权要求
- 参数表：名称、位置、类型、必填、说明
- 响应：成功与典型错误的 JSON 示例
- 至少一个完整 curl 示例

## 原则

- 命令必须可直接复制执行；版本号、占位符用尖括号标注
- 示例优先于描述；能展示真实输出就不要空谈
- 保持与代码同步：发现文档与实现不符时先指出冲突再修改
- 文档语言与项目现有文档保持一致
`,
	},
}
