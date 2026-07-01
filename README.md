# DeepSeek-Reasonix Bug Issues 报告工具

本地运行的工具，用于整理、分类、优先排序 [esengine/DeepSeek-Reasonix](https://github.com/esengine/DeepSeek-Reasonix) 的 GitHub Issues，生成可交互的网页报告。

## 快速开始

```bash
cd d:\github-clone\reasonix-bug-issues
go build -o reasonix-bug-report.exe .
./reasonix-bug-report.exe
```

浏览器访问 http://localhost:8765/

## 常用命令

```bash
# 仅拉取增量数据
./reasonix-bug-report.exe --fetch

# 全量拉取（忽略增量时间戳，需注意 API 限速）
./reasonix-bug-report.exe --fetch --full

# 仅启动服务（使用已有数据库）
./reasonix-bug-report.exe --serve

# 更新分类逻辑后重新分类已有数据（无需联网）
./reasonix-bug-report.exe --reclassify --serve

# 自动标记有关联 PR 的 issue 为"已有人跟进"
./reasonix-bug-report.exe --tag-prs --serve

# 指定保留的 issue 状态（默认 open，可选 closed / all）
./reasonix-bug-report.exe --fetch --state open
```

## 配置

支持 `.env` 文件配置 `GITHUB_TOKEN`（可选）：

```bash
cp .env.example .env
# 编辑 .env，填入你的 GitHub Token
GITHUB_TOKEN=ghp_xxxxxxxxxxxx
```

Token 用于提升 GitHub API 请求频率限制。未配置时仍可使用，但可能遇到限速。

## 功能

- **分类**：按 6 个功能模块自动分类（标签 + 标题关键词双重匹配）
- **优先级**：量化评分 → P0/P1/P2/P3
- **PR 关联**：自动匹配关联的 Pull Request
- **手动标记**：已修复待确认、已有人跟进、计划修复
- **筛选 & 图表**：按优先级 / 模块 / 标记 / 关键字筛选，含分布图与环形图
- **导入/导出**：标记数据 JSON 备份

## 数据更新行为

- 点击页面"刷新数据"按钮或 `--fetch` 触发增量拉取
- 增量同步始终以 `state=all` + `since` 拉取，能感知 issue 关闭事件
- 同步后自动清理 DB 中状态不匹配 `--state` 的 issue（默认清理已关闭项）；全量同步还会回收 GitHub 上已删除的 issue
- 标记数据（`issue_tags`）独立于 GitHub 源数据，不会因刷新覆盖

## 技术栈

Go 1.21+ · SQLite (`modernc.org/sqlite`) · 原生 HTML/CSS/JavaScript

## 目录结构

```
.
├── main.go           # 入口：CLI + HTTP 服务器
├── internal/         # Go 后端（types / db / github / handler / dotenv）
├── templates/        # HTML 模板
├── static/           # CSS / JS
└── data/             # SQLite 数据库（自动创建）
```
