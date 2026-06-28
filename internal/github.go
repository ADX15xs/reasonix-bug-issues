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
	"config-setup":  {"配置 & 更新", "⚙️", []string{"config", "updater"}},
	"platform":       {"平台特定", "🏷️", []string{}},
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
	"enhancement":    {"增强", "✨"},
	"documentation": {"文档", "📚"},
	"question":       {"讨论", "❓"},
	"dependencies":   {"依赖", "📦"},
	"good first issue": {"Good First Issue", "🌱"},
	"help wanted":     {"Help Wanted", "🙏"},
	"other":          {"其他", "📋"},
}

var PriorityLabels = map[string]struct {
	Name  string
	Label string
	CSS   string
	Color string
}{
	"P0":            {"P0", "P0 致命", "priority-p0", "#f85149"},
	"P1":            {"P1", "P1 高优", "priority-p1", "#f0883e"},
	"P2":            {"P2", "P2 中优", "priority-p2", "#d29922"},
	"P3":            {"P3", "P3 低优", "priority-p3", "#8b949e"},
	"enhancement":   {"增强", "增强", "priority-enhancement", "#a371f7"},
	"question":      {"讨论", "讨论", "priority-question", "#58a6ff"},
	"documentation": {"文档", "文档", "priority-documentation", "#58a6ff"},
	"dependencies":  {"依赖", "依赖", "priority-other", "#8b949e"},
	"other":         {"其他", "其他", "priority-other", "#8b949e"},
}

var TagMeta = map[string]struct {
	Name  string
	CSS   string
	Color string
}{
	"none":      {"未标记", "tag-none", "#8b949e"},
	"fixed":     {"已修复待确认", "tag-fixed", "#238636"},
	"following": {"已有人跟进", "tag-following", "#1f6feb"},
	"ignored":   {"已 review", "tag-ignored", "#6e7681"},
}

var PriorityOrderMap = map[string]int{
	"P0": 0, "P1": 1, "P2": 2, "P3": 3,
	"enhancement": 4, "question": 5, "documentation": 6,
	"dependencies": 7, "other": 8,
}

type GitHubIssue struct {
	Number      int       `json:"number"`
	Title      string    `json:"title"`
	HTMLURL    string    `json:"html_url"`
	Body       string    `json:"body"`
	User       struct {
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

func FetchAllIssues(state string) ([]GitHubIssue, error) {
	var all []GitHubIssue
	page := 1
	for {
		var issues []GitHubIssue
		data, err := fetchGitHub(fmt.Sprintf("%s/repos/%s/%s/issues?state=%s&page=%d&per_page=100",
			GitHubAPI, RepoOwner, RepoName, state, page))
		if err != nil {
			return nil, err
		}
		if len(data) == 0 {
			break
		}
		if err := json.Unmarshal(data, &issues); err != nil {
			return nil, err
		}
		// Filter out PRs
		for _, iss := range issues {
			if iss.Title == "" && iss.HTMLURL == "" {
				continue
			}
			all = append(all, iss)
		}
		if len(issues) < 100 {
			break
		}
		page++
	}
	return all, nil
}

func FetchAllPRs(state string) ([]GitHubPR, error) {
	var all []GitHubPR
	page := 1
	for {
		var prs []GitHubPR
		data, err := fetchGitHub(fmt.Sprintf("%s/repos/%s/%s/pulls?state=%s&page=%d&per_page=100",
			GitHubAPI, RepoOwner, RepoName, state, page))
		if err != nil {
			return nil, err
		}
		if len(data) == 0 {
			break
		}
		if err := json.Unmarshal(data, &prs); err != nil {
			return nil, err
		}
		all = append(all, prs...)
		if len(prs) < 100 {
			break
		}
		page++
	}
	return all, nil
}

func fetchGitHub(url string) ([]byte, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "Reasonix-Bug-Report-Generator")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
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
	category := determineCategory(labelNames, isBug)
	priorityOrder := PriorityOrderMap[priority]

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
		PriorityOrder: priorityOrder,
		Category:      category,
	}
}

func FindRelatedPRs(issueNumber int, issueTitle string, prs []GitHubPR) []RelatedPR {
	related := []RelatedPR{}
	issueTitleLower := strings.ToLower(issueTitle)

	for _, pr := range prs {
		body := strings.ToLower(pr.Body)
		title := strings.ToLower(pr.Title)
		numStr := strconv.Itoa(issueNumber)

		// Check references
		isRelated := false

		if strings.Contains(body, "#"+numStr) || strings.Contains(title, "#"+numStr) {
			isRelated = true
		}

		fixRE := regexp.MustCompile(fmt.Sprintf(`\bfix(?:es|ed)?\s*#?%s\b`, numStr))
		closeRE := regexp.MustCompile(fmt.Sprintf(`\bclose(?:s|d)?\s*#?%s\b`, numStr))
		resolveRE := regexp.MustCompile(fmt.Sprintf(`\bresolve(?:s|d)?\s*#?%s\b`, numStr))

		if fixRE.MatchString(body) || fixRE.MatchString(title) ||
			closeRE.MatchString(body) || closeRE.MatchString(title) ||
			resolveRE.MatchString(body) || resolveRE.MatchString(title) {
			isRelated = true
		}

		if strings.Contains(issueTitle, pr.Title) ||
			strings.Contains(title, issueTitleLower) {
			isRelated = true
		}

		if isRelated {
			related = append(related, RelatedPR{
				Number:  pr.Number,
				Title:   pr.Title,
				HTMLURL: pr.HTMLURL,
			})
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
	if !isBug {
		if contains(labels, "enhancement") {
			return "enhancement"
		}
		if contains(labels, "question") {
			return "question"
		}
		if contains(labels, "documentation") {
			return "documentation"
		}
		if contains(labels, "dependencies") {
			return "dependencies"
		}
		return "other"
	}

	severity := 0
	for _, t := range labels {
		if v, ok := SeverityScore[t]; ok && v > severity {
			severity = v
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

func determineCategory(labels []string, isBug bool) string {
	if !isBug {
		if contains(labels, "enhancement") {
			return "enhancement"
		}
		if contains(labels, "documentation") {
			return "documentation"
		}
		if contains(labels, "question") {
			return "question"
		}
		if contains(labels, "dependencies") {
			return "dependencies"
		}
		if contains(labels, "good first issue") {
			return "good first issue"
		}
		if contains(labels, "help wanted") {
			return "help wanted"
		}
		return "other"
	}

	for key, cat := range BugCategories {
		for _, tag := range cat.Tags {
			if contains(labels, tag) {
				return key
			}
		}
	}
	return "platform"
}
