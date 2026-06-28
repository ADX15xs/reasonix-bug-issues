package internal

import "time"

const (
	TagNone      = "none"
	TagFixed     = "fixed"
	TagFollowing = "following"
	TagIgnored   = "ignored"

	PriorityP0 = "P0"
	PriorityP1 = "P1"
	PriorityP2 = "P2"
	PriorityP3 = "P3"
)

type Issue struct {
	ID          int64     `json:"id"`
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	HTMLURL     string    `json:"html_url"`
	Body        string    `json:"body,omitempty"`
	UserLogin   string    `json:"user_login"`
	UserAvatar  string    `json:"user_avatar"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Comments    int       `json:"comments"`
	Labels      []Label   `json:"labels"`
	Priority    string    `json:"priority"`
	PriorityOrder int     `json:"priority_order"`
	Category    string    `json:"category"`
	Tag         string    `json:"tag"`
}

type Label struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

type RelatedPR struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	HTMLURL string `json:"html_url"`
}

type IssueDetail struct {
	Issue
	RelatedPRs []RelatedPR `json:"related_prs"`
}

type Stats struct {
	TotalIssues     int            `json:"total_issues"`
	BugCount        int            `json:"bug_count"`
	EnhancementCount int            `json:"enhancement_count"`
	PriorityCounts  map[string]int `json:"priority_counts"`
	CategoryCounts  map[string]int `json:"category_counts"`
}

type FilterParams struct {
	Priorities []string `json:"priorities"`
	Category   string   `json:"category"`
	Tag        string   `json:"tag"`
	Search     string   `json:"search"`
	Sort       string   `json:"sort"`
	Page       int      `json:"page"`
	PageSize   int      `json:"page_size"`
}

type PageResult struct {
	Items      []IssueDetail `json:"items"`
	Total      int           `json:"total"`
	Page       int           `json:"page"`
	PageSize   int           `json:"page_size"`
	PageCount  int           `json:"page_count"`
	Stats      Stats         `json:"stats"`
}
