---
name: "reasonix-dashboard-gen"
description: "Generates a cute Chinese infographic image visualizing Reasonix GitHub bug issues by module and priority. Invoke when user asks to generate/update the bug dashboard image or refresh the visual report."
---

# Reasonix Bug Dashboard Image Generator

从本地 Go 应用获取已分类的 Issues 统计数据，生成一张可爱的中文可视化信息图，覆盖保存到项目目录。

## 触发条件

用户提到以下任意一种场景时调用本 skill：
- "生成 bug 可视化图"
- "更新 dashboard 图片"
- "重新生成图表"
- "刷新报告图片"
- 任何与生成/更新 Reasonix bug 可视化图片相关的请求

## 执行步骤

### 1. 确保本地数据最新

项目根目录为 `d:\github-clone\reasonix-bug-issues`。

先检查 `.env` 文件中是否有 `GITHUB_TOKEN`，如有则 API 限额更高。然后运行 Go 应用拉取最新数据：

```powershell
cd d:\github-clone\reasonix-bug-issues
.\reasonix-bug-report.exe --fetch
```

> 如果 `reasonix-bug-report.exe` 不存在，需先编译：`go build -o reasonix-bug-report.exe .`

### 2. 启动本地服务器并获取统计数据

启动 HTTP 服务器（端口 8765），然后请求 `/api/stats` 端点获取统计信息：

```powershell
cd d:\github-clone\reasonix-bug-issues
.\reasonix-bug-report.exe --serve
```

在另一个终端请求统计数据：

```powershell
curl http://localhost:8765/api/stats
```

**Stats API 返回的 JSON 结构：**

```json
{
  "total_issues": 123,
  "bug_count": 80,
  "enhancement_count": 15,
  "priority_counts": {
    "P0": 3,
    "P1": 12,
    "P2": 35,
    "P3": 73
  },
  "category_counts": {
    "agent-core": 20,
    "ui-experience": 15,
    "model-provider": 10,
    "integration": 12,
    "config-setup": 8,
    "platform": 15,
    "other": 43
  }
}
```

获取到数据后停止服务器。

### 3. 分类规则说明（供参考，实际分类由 Go 应用完成）

Go 应用的分类逻辑（`internal/github.go`）：

| 模块 | 标签匹配 | 关键词匹配 |
|------|----------|------------|
| 🧠 Agent 核心 | `agent`, `memory` | agent, memory, context, thinking, reasoning, cot, prompt, tool call, subagent, task, plan |
| 🖥️ UI 交互 | `desktop`, `tui`, `rendering` | ui, display, render, layout, css, theme, scroll, window, dialog, button, terminal, tui, desktop, gui, electron, frontend, markdown, html, notification, popup, modal, icon, cursor, resize, drag, drop, clipboard, copy, paste, input, text, editor, code block, highlight, syntax |
| 🔌 模型与供应商 | `provider` | model, provider, api, llm, openai, claude, anthropic, deepseek, gpt, api key, rate limit, streaming, token, endpoint, timeout, quota, gemini, ollama, groq, mistral, bedrock, vertex, azure, openrouter |
| 🔗 集成与插件 | `mcp`, `skills` | mcp, plugin, skill, integrat, connect, extension, addon, hook, webhook, server, protocol, tool, mcp server, vscode, jetbrains, intellij, ide, git, github, gitlab, bitbucket, slack, discord, notion, jira, linear, teams, telegram, wechat |
| ⚙️ 配置与更新 | `config`, `updater` | config, install, update, upgrade, setup, deploy, build, compile, version, changelog, release, docker, pip, npm, binary, executable, path, permission, dependenc, package, library, module, import, export, init, bootstrap, startup, launch, crash, error, fail, exception, panic, fatal |
| 🏷️ 平台特定 | 仅有平台标签，无上述功能标签 | windows, mac, linux, platform, os, ios, android, macos, win32, win64, x86, arm, apple silicon, wsl, ubuntu, debian, fedora, arch, cross platform, powershell, cmd, bash, zsh, shell, terminal, homebrew, choco, scoop, winget, apt, yum, dnf |
| 📋 其他 | 无匹配 | — |

### 4. 优先级量化打分规则（供参考，实际打分由 Go 应用完成）

总分 = 严重性分 + 模块关键度分 + 活跃度分

**严重性分：**
- crash = 10
- data-loss = 9
- security = 8
- blocked-upstream = 6
- 非 bug issue 严重性分为 0

**模块关键度分：**
- agent = 5
- memory / desktop / provider = 4
- tui / mcp / skills / config / rendering = 3
- updater = 2
- 其他 = 1

**活跃度分：**
- 评论 >= 3：+2
- 评论 >= 1：+1
- 7 天内更新过：+2

**等级映射：**
- P0 致命：总分 >= 14
- P1 高优：总分 >= 8
- P2 中优：总分 >= 4
- P3 低优：总分 < 4

### 5. 生成可视化图片（优先使用 bash 脚本）

**方法一：使用 bash 脚本（推荐）**

项目根目录已提供 `generate_image.sh` 脚本，封装了 SenseNova 文生图 API 调用：

```bash
cd d:\github-clone\reasonix-bug-issues
bash generate_image.sh dashboard_prompt.json -o reasonix-bug-dashboard-cn.jpg
```

**使用方式：**

1. 根据从 `/api/stats` 获取的统计数据，构建 JSON prompt 文件 `dashboard_prompt.json`：

```json
{
  "model": "sensenova-u1-fast",
  "prompt": "[PURPOSE]: 可爱的数据可视化信息图，用于展示 GitHub Bug Issues 分布。深色背景(#0d1117风格)，扁平可爱插画风格。画面内容：左上角是品牌名\"Reasonix\"和小蜜蜂logo；中间是一个大大的彩色甜甜圈图，展示各模块Bug数量：Agent核心[实际数量]个🧠、UI交互[实际数量]个🖥️、模型供应商[实际数量]个🔌、集成插件[实际数量]个🔗、配置更新[实际数量]个⚙️、平台特定[实际数量]个🏷️；每个模块旁边配有可爱小图标。右侧是优先级横向条形图：P0致命[实际数量]个带火焰表情、P1高优[实际数量]个带闪电、P2中优[实际数量]个带小笑脸、P3低优[实际数量]个带云朵表情。顶部大字展示：[总Issues数]个总Issues、[Bug数]个Bug待处理。整体kawaii风格，中文标签，圆润字体，马卡龙配色，信息清晰且视觉有趣。",
  "size": "2752x1536",
  "n": 1
}
```

> **注意**：将 prompt 中的 `[实际数量]`、`[总Issues数]`、`[Bug数]` 替换为从 `/api/stats` 获取的实际数据。只展示 bug_count 中涉及的模块（agent-core, ui-experience, model-provider, integration, config-setup, platform），不展示 other。

2. 运行脚本生成图片（使用 `-o` 参数指定输出文件名，直接覆盖）：

```bash
bash generate_image.sh dashboard_prompt.json -o reasonix-bug-dashboard-cn.jpg
```

脚本会自动：
- 从 `.env` 加载 `SENSENOVA_API_KEY`
- 调用 SenseNova 文生图 API
- 下载生成的图片并直接保存为指定的文件名（覆盖旧文件）

**脚本参数说明：**
- `-o filename` 或 `--output filename`：指定输出文件名，直接覆盖保存
- 不指定 `-o` 时，默认生成 `generated_image_YYYYMMDD_HHMMSS.png`

**方法二：使用 GenerateImage 工具（备选）**

如果 bash 脚本不可用或需要更精细控制，可使用 GenerateImage 工具：

**Prompt 模板：**

```
[PURPOSE]: 可爱的数据可视化信息图，用于展示 GitHub Bug Issues 分布。深色背景(#0d1117风格)，扁平可爱插画风格。画面内容：左上角是品牌名"Reasonix"和小蜜蜂logo；中间是一个大大的彩色甜甜圈图，展示各模块Bug数量：Agent核心[N]个🧠、UI交互[N]个🖥️、模型供应商[N]个🔌、集成插件[N]个🔗、配置更新[N]个⚙️、平台特定[N]个🏷️；每个模块旁边配有可爱小图标。右侧是优先级横向条形图：P0致命[N]个带火焰表情、P1高优[N]个带闪电、P2中优[N]个带小笑脸、P3低优[N]个带云朵表情。顶部大字展示：[N]个总Issues、[N]个Bug待处理。整体kawaii风格，中文标签，圆润字体，马卡龙配色，信息清晰且视觉有趣。
```

> **注意**：将 Prompt 中的 `[N]` 替换为从 `/api/stats` 获取的实际数据。只展示 `bug_count` 中涉及的模块（agent-core, ui-experience, model-provider, integration, config-setup, platform），不展示 other。

**图片参数：**
- image_size: `landscape_16_9`
- path: `d:\github-clone\reasonix-bug-issues\reasonix-bug-dashboard-cn`

### 6. 覆盖保存

使用 `-o` 参数时，脚本会自动将生成的图片直接保存为指定文件名，覆盖旧版本。无需额外的重命名步骤。

## 输出

生成完成后，向用户汇报：
1. 图片已生成的确认
2. 关键数据的简要摘要（总 Issues、Bug 数、各模块分布、优先级分布）
3. 图片文件路径
