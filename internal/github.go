package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	RepoOwner = "esengine"
	RepoName  = "DeepSeek-Reasonix"
	GitHubAPI = "https://api.github.com"
)

var SeverityScore = map[string]int{
	"crash": 10, "data-loss": 9, "security": 8, "blocked-upstream": 6,
}

var ModuleScore = map[string]int{
	"agent": 5, "memory": 4, "desktop": 4, "provider": 4,
	"tui": 3, "mcp": 3, "skills": 3, "config": 3, "rendering": 3, "updater": 2,
}

var BugCategories = map[string]struct {
	Name  string
	Emoji string
	Tags  []string
}{
	"agent-core":     {"Agent 核心", "🧠", []string{"agent", "memory"}},
	"ui-experience":  {"UI 交互", "🖥️", []string{"desktop", "tui", "rendering"}},
	"model-provider": {"模型 & 供应商", "🔌", []string{"provider"}},
	"integration":    {"集成 & 插件", "🔗", []string{"mcp", "skills"}},
	"config-setup":   {"配置 & 更新", "⚙️", []string{"config", "updater"}},
	"platform":       {"平台特定", "🏷️", []string{}},
}

// CategoryKeywords provides title-based fallback classification when no label tags match.
var CategoryKeywords = map[string][]string{
	"agent-core": {
		"agent", "memory", "context", "thinking", "reasoning",
		"chain of thought", "cot", "prompt", "tool call", "tool use",
		"subagent", "task", "plan", "planner",
	},
	"ui-experience": {
		"ui", "display", "render", "layout", "css", "style", "font",
		"界面", "显示", "theme", "scroll", "window", "dialog", "button",
		"click", "hover", "tooltip", "sidebar", "panel", "viewport",
		"dark mode", "light mode", "terminal", "tui", "desktop", "gui",
		"app", "electron", "frontend", "front-end", "markdown", "html",
		"notification", "popup", "modal", "icon", "cursor", "resize",
		"drag", "drop", "clipboard", "copy", "paste", "input", "text",
		"editor", "code block", "highlight", "syntax",
	},
	"model-provider": {
		"model", "provider", "api", "llm", "openai", "claude", "anthropic",
		"deepseek", "gpt", "api key", "rate limit", "streaming", "token",
		"endpoint", "请求", "超时", "timeout", "quota", "gemini", "ollama",
		"groq", "mistral", "bedrock", "vertex", "azure", "openrouter",
		"temperature", "top_p", "frequency", "presence", "max_token",
		"stop sequence", "chat completion", "embedding",
	},
	"integration": {
		"mcp", "plugin", "skill", "integrat", "connect", "extension",
		"addon", "hook", "webhook", "server", "protocol", "tool",
		"mcp server", "vscode", "jetbrains", "intellij", "ide",
		"git", "github", "gitlab", "bitbucket", "slack", "discord",
		"notion", "jira", "linear", "teams", "telegram", "wechat",
	},
	"config-setup": {
		"config", "install", "update", "upgrade", "setup", "deploy",
		"build", "compile", "version", "changelog", "release", "docker",
		"pip", "npm", "binary", "executable", "环境变量", "path",
		"permission", "dependenc", "package", "library", "module",
		"import", "export", "init", "bootstrap", "startup", "launch",
		"crash", "error", "fail", "exception", "panic", "fatal",
		"环境", "安装", "更新", "配置", "启动", "失败", "报错", "错误",
	},
	"platform": {
		"windows", "mac", "linux", "platform", "os", "ios", "android",
		"macos", "mac os", "win32", "win64", "x86", "arm", "apple silicon",
		"wsl", "ubuntu", "debian", "fedora", "arch", "cross platform",
		"powershell", "cmd", "bash", "zsh", "shell", "terminal",
		"homebrew", "choco", "scoop", "winget", "apt", "yum", "dnf",
		"registry", "system32", "program files", "appdata", ".app",
	},
}

var CategoryMeta = map[string]struct {
	Name  string
	Emoji string
}{
	"agent-core":     {"Agent 核心", "🧠"},
	"ui-experience":  {"UI 交互", "🖥️"},
	"model-provider": {"模型 & 供应商", "🔌"},
	"integration":    {"集成 & 插件", "🔗"},
	"config-setup":   {"配置 & 更新", "⚙️"},
	"platform":       {"平台特定", "🏷️"},
	"other":          {"其他", "📋"},
}

var PriorityLabels = map[string]struct {
	Name  string
	Label string
	CSS   string
	Color string
}{
	"P0": {"P0", "P0 致命", "priority-p0", "#cf222e"},
	"P1": {"P1", "P1 高优", "priority-p1", "#bf3989"},
	"P2": {"P2", "P2 中优", "priority-p2", "#9a6700"},
	"P3": {"P3", "P3 低优", "priority-p3", "#6e7781"},
}

var TagMeta = map[string]struct {
	Name  string
	CSS   string
	Color string
}{
	"none":      {"未标记", "tag-none", "#8b949e"},
	"fixed":     {"已修复待确认", "tag-fixed", "#238636"},
	"following": {"已有人跟进", "tag-following", "#1f6feb"},
	"planned":   {"计划修复", "tag-planned", "#a371f7"},
}

var PriorityOrderMap = map[string]int{
	"P0": 0, "P1": 1, "P2": 2, "P3": 3,
}

type GitHubIssue struct {
	Number     int `json:"number"`
	Title     string `json:"title"`
	HTMLURL   string `json:"html_url"`
	Body      string `json:"body"`
	State     string `json:"state"`
	User      struct {
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
	} `json:"user"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Comments  int    `json:"comments"`
	Labels    []struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	} `json:"labels"`
}

type GitHubPR struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	HTMLURL   string `json:"html_url"`
	Body      string `json:"body"`
	State     string `json:"state"`
	UpdatedAt string `json:"updated_at"`
}

// FetchIssuesParams controls what to fetch.
type FetchIssuesParams struct {
	State string // "open", "closed", or "all"
	Since string // RFC3339 timestamp; if set, only issues updated after this time are returned
}

func FetchIssues(params FetchIssuesParams) ([]GitHubIssue, error) {
	var all []GitHubIssue
	url := fmt.Sprintf("%s/repos/%s/%s/issues?state=%s&per_page=100&pull_requests=false",
		GitHubAPI, RepoOwner, RepoName, params.State)
	if params.Since != "" {
		url += "&since=" + params.Since
	}

	for url != "" {
		data, nextURL, err := fetchGitHubWithLink(url)
		if err != nil {
			return nil, err
		}
		if len(data) == 0 {
			break
		}
		var issues []GitHubIssue
		if err := json.Unmarshal(data, &issues); err != nil {
			return nil, err
		}
		if len(issues) == 0 {
			break
		}
		all = append(all, issues...)
		url = nextURL
	}
	return all, nil
}

func FetchAllPRs(state string) ([]GitHubPR, error) {
	var all []GitHubPR
	url := fmt.Sprintf("%s/repos/%s/%s/pulls?state=%s&per_page=100",
		GitHubAPI, RepoOwner, RepoName, state)

	for url != "" {
		data, nextURL, err := fetchGitHubWithLink(url)
		if err != nil {
			return nil, err
		}
		if len(data) == 0 {
			break
		}
		var prs []GitHubPR
		if err := json.Unmarshal(data, &prs); err != nil {
			return nil, err
		}
		all = append(all, prs...)
		url = nextURL
	}
	return all, nil
}

func fetchGitHubWithLink(url string) ([]byte, string, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "Reasonix-Bug-Report-Generator")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 304 {
		return nil, "", nil
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("GitHub API error %d: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	nextURL := parseNextLink(resp.Header.Get("Link"))
	return data, nextURL, nil
}

// parseNextLink extracts the next page URL from the Link header.
func parseNextLink(header string) string {
	for _, part := range strings.Split(header, ",") {
		if strings.Contains(part, `rel="next"`) {
			start := strings.Index(part, "<")
			end := strings.Index(part, ">")
			if start >= 0 && end > start {
				return part[start+1 : end]
			}
		}
	}
	return ""
}

func fetchGitHub(url string) ([]byte, error) {
	data, _, err := fetchGitHubWithLink(url)
	return data, err
}

func ToInternalIssue(ghIss GitHubIssue) Issue {
	labels := make([]Label, len(ghIss.Labels))
	labelNames := []string{}
	for i, l := range ghIss.Labels {
		labels[i] = Label{Name: l.Name, Color: l.Color}
		labelNames = append(labelNames, strings.ToLower(l.Name))
	}

	createdAt, _ := time.Parse(time.RFC3339, ghIss.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339, ghIss.UpdatedAt)

	isBug := contains(labelNames, "bug")
	priority := determinePriority(labelNames, isBug, ghIss.Comments, ghIss.UpdatedAt)
	category := determineCategory(labelNames, ghIss.Title, ghIss.Body)

	return Issue{
		ID:            int64(ghIss.Number),
		Number:        ghIss.Number,
		Title:         ghIss.Title,
		HTMLURL:       ghIss.HTMLURL,
		Body:          ghIss.Body,
		UserLogin:     ghIss.User.Login,
		UserAvatar:    ghIss.User.AvatarURL,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
		Comments:      ghIss.Comments,
		Labels:        labels,
		Priority:      priority,
		PriorityOrder: PriorityOrderMap[priority],
		Category:      category,
	}
}

func FindRelatedPRs(issueNumber int, issueTitle string, prs []GitHubPR) []RelatedPR {
	related := []RelatedPR{}
	issueTitleLower := strings.ToLower(issueTitle)
	numStr := strconv.Itoa(issueNumber)

	for _, pr := range prs {
		body := strings.ToLower(pr.Body)
		title := strings.ToLower(pr.Title)

		if strings.Contains(body, "#"+numStr) || strings.Contains(title, "#"+numStr) {
			related = append(related, RelatedPR{Number: pr.Number, Title: pr.Title, HTMLURL: pr.HTMLURL})
			continue
		}

		fixRE := regexp.MustCompile(fmt.Sprintf(`\bfix(?:es|ed)?\s*#?%s\b`, numStr))
		closeRE := regexp.MustCompile(fmt.Sprintf(`\bclose(?:s|d)?\s*#?%s\b`, numStr))
		resolveRE := regexp.MustCompile(fmt.Sprintf(`\bresolve(?:s|d)?\s*#?%s\b`, numStr))

		if fixRE.MatchString(body) || fixRE.MatchString(title) ||
			closeRE.MatchString(body) || closeRE.MatchString(title) ||
			resolveRE.MatchString(body) || resolveRE.MatchString(title) {
			related = append(related, RelatedPR{Number: pr.Number, Title: pr.Title, HTMLURL: pr.HTMLURL})
			continue
		}

		if strings.Contains(issueTitle, pr.Title) || strings.Contains(title, issueTitleLower) {
			related = append(related, RelatedPR{Number: pr.Number, Title: pr.Title, HTMLURL: pr.HTMLURL})
		}
	}
	return related
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func determinePriority(labels []string, isBug bool, comments int, updatedAt string) string {
	severity := 0
	if isBug {
		for _, t := range labels {
			if v, ok := SeverityScore[t]; ok && v > severity {
				severity = v
			}
		}
	}

	module := 1
	for _, t := range labels {
		if v, ok := ModuleScore[t]; ok && v > module {
			module = v
		}
	}

	activity := 0
	if comments >= 3 {
		activity += 2
	} else if comments >= 1 {
		activity += 1
	}
	if updatedAt != "" {
		if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
			if time.Since(t).Hours() <= 24*7 {
				activity += 2
			}
		}
	}

	total := severity + module + activity
	if total >= 14 {
		return "P0"
	}
	if total >= 8 {
		return "P1"
	}
	if total >= 4 {
		return "P2"
	}
	return "P3"
}

func determineCategory(labels []string, title, body string) string {
	// First pass: try label tags
	for key, cat := range BugCategories {
		for _, tag := range cat.Tags {
			if contains(labels, tag) {
				return key
			}
		}
	}

	// Second pass: try title/body keyword matching (limit body to 2000 chars for performance)
	text := strings.ToLower(title + " ")
	if len(body) > 2000 {
		text += strings.ToLower(body[:2000])
	} else {
		text += strings.ToLower(body)
	}
	for _, catKey := range []string{"agent-core", "ui-experience", "model-provider", "integration", "config-setup", "platform"} {
		for _, kw := range CategoryKeywords[catKey] {
			if strings.Contains(text, strings.ToLower(kw)) {
				return catKey
			}
		}
	}

	return "other"
}
