# DeepSeek-Reasonix Bug Issues 报告工具

本地运行的工具，用于整理、分类、优先排序 [esengine/DeepSeek-Reasonix](https://github.com/esengine/DeepSeek-Reasonix) 的 GitHub Issues，生成可交互的网页报告。

## 快速开始

```bash
cd d:\github-clone\reasonix-bug-issues
go build -o reasonix-bug-report.exe .
./reasonix-bug-report.exe
```

浏览器访问 http://localhost:8765/

## 使用方式

```bash
# 首次运行：拉取数据 + 启动服务
./reasonix-bug-report.exe

# 仅拉取增量数据
./reasonix-bug-report.exe --fetch

# 全量拉取（忽略增量时间戳，需注意 API 限速）
./reasonix-bug-report.exe --fetch --full

# 仅启动服务（使用已有数据库）
./reasonix-bug-report.exe --serve

# 更新分类逻辑后重新分类已有数据（无需联网）
./reasonix-bug-report.exe --reclassify --serve
```

## 功能

- **分类**：Issue 按 6 个功能模块自动分类（Agent 核心、UI 交互、模型供应商、集成插件、配置更新、平台特定），通过 GitHub 标签 + 标题关键词双重匹配
- **优先级**：量化评分 → P0/P1/P2/P3，所有 issue 统一算法
- **PR 关联**：自动匹配关联的 Pull Request
- **手动标记**：已修复待确认、已有人跟进、计划修复
- **筛选**：按优先级、模块、标记状态、关键字搜索
- **图表**：优先级分布、模块分布、标记状态环形图
- **导入/导出**：标记数据 JSON 备份

## 数据更新

- 点击页面"刷新数据"按钮触发增量拉取
- 标记数据独立于 GitHub 源数据，不会因刷新覆盖
- 增量同步基于 GitHub API `since` 参数，只拉取变化的 issue

## 技术栈

Go 1.21+ · SQLite · 原生 HTML/CSS/JavaScript

## 目录结构

```
.
├── main.go           # 入口
├── internal/         # Go 后端
├── templates/        # HTML 模板
├── static/           # CSS / JS
└── data/             # SQLite 数据库
```