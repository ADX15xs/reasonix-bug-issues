# DeepSeek-Reasonix Bug Issues 报告工具

## 项目概述

用于整理、分类、优先排序 GitHub Issues 的本地报告工具。

## 技术栈

- **后端**: Go 1.21+, SQLite (`modernc.org/sqlite` 纯 Go 实现)
- **前端**: 原生 HTML/CSS/JavaScript（无框架依赖）
- **数据存储**: SQLite（包含 issues 表、关联 PR 表、标记表、元数据表）

## 快速开始

```bash
# 首次运行：拉取 GitHub 数据并启动服务器
go build -o reasonix-bug-report.exe .
./reasonix-bug-report.exe

# 仅拉取数据（不启动服务器）
./reasonix-bug-report.exe --fetch

# 仅启动服务器（使用已有数据库）
./reasonix-bug-report.exe --serve

# 全量拉取（忽略增量时间戳）
./reasonix-bug-report.exe --fetch --full

# 重新分类已有数据（无需联网）
./reasonix-bug-report.exe --reclassify --serve
```

访问 http://localhost:8765/

## 目录结构

```
.
├── main.go           # 入口，CLI + HTTP 服务器
├── internal/
│   ├── types.go      # 数据结构
│   ├── db.go         # SQLite 操作
│   ├── github.go     # GitHub API 获取与分类
│   └── handler.go    # HTTP 路由与 API 处理器
├── templates/
│   └── index.html    # HTML 模板（引用 /static/）
├── static/
│   ├── style.css     # 样式（亮色主题）
│   └── app.js        # 前端交互逻辑
├── data/             # SQLite 数据库文件（自动创建）
└── .gitignore
```

## CLI 参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `--port` | HTTP 监听端口 | 8765 |
| `--db` | SQLite 数据库路径 | `data/reasonix.db` |
| `--fetch` | 仅拉取数据，不启动服务器 | false |
| `--serve` | 仅启动服务器，跳过数据拉取 | false |
| `--full` | 强制全量同步（忽略增量时间戳） | false |
| `--reclassify` | 对已有数据重新分类和评分（无需联网） | false |

## 功能

- **分类**: Issue 按功能模块分类（Agent 核心、UI 交互、模型供应商、集成插件、配置更新、平台特定、其他），通过标签 + 标题关键词双重匹配
- **优先级**: 量化打分（严重性 + 模块关键度 + 活跃度）→ P0/P1/P2/P3，所有 issue 统一评分
- **PR 关联**: 自动匹配 Fix/Close/Resolve 关键字和标题相似度
- **手动标记**: 三种状态（已修复待确认、已有人跟进、计划修复），存储于 SQLite
- **筛选**: 按优先级、模块、标记状态、关键字搜索
- **图表**: 优先级分布图、模块分布图、标记状态环形图
- **导入/导出**: 标记数据 JSON 文件

## API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/` | GET | 返回 HTML 页面 |
| `/api/issues` | GET | 查询 issues（支持分页、筛选、排序） |
| `/api/issues/tags` | POST | 设置 issue 标记 |
| `/api/stats` | GET | 返回统计数据 |
| `/api/tags/export` | GET | 导出所有标记为 JSON |
| `/api/tags/import` | POST | 从 JSON 导入标记 |
| `/refresh` | POST | 触发后台增量拉取 GitHub 数据 |

## 数据库 Schema

- `issues`: id, number, title, html_url, body, user_login, user_avatar, created_at, updated_at, comments, labels(JSON), priority, priority_order, category
- `related_prs`: id, issue_number, pr_number, pr_title, pr_html_url
- `issue_tags`: issue_number(PK), tag, updated_at
- `meta`: key(PK), value

## 数据刷新

手动增量更新：点击页面"刷新数据"按钮或运行 `./reasonix-bug-report.exe --fetch`。标记数据（issue_tags）独立于 GitHub 源数据，不会被覆盖。

## 分类与优先级规则

### 分类（模块）

分类分两级：
1. **标签匹配**：通过 GitHub labels 直接匹配模块
2. **关键词匹配**：标签未命中时，通过标题和正文关键词推断

### 优先级（评分）

```
总分 = 严重性分 + 模块关键度分 + 活跃度分

P0: 总分 >= 14
P1: 总分 >= 8
P2: 总分 >= 4
P3: 总分 < 4
```

- 严重性分：基于标签（crash/bug/data-loss 等），非 bug issue 为 0
- 模块关键度分：基于模块重要性
- 活跃度分：评论数 + 7 日内更新