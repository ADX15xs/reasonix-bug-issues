package internal

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	db *sql.DB
}

func OpenDB(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path+"?_busy_timeout=30000&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	d := &DB{db: db}
	if err := d.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return d, nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS issues (
		id INTEGER PRIMARY KEY,
		number INTEGER UNIQUE NOT NULL,
		title TEXT NOT NULL,
		html_url TEXT NOT NULL,
		body TEXT DEFAULT '',
		user_login TEXT NOT NULL,
		user_avatar TEXT NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		comments INTEGER DEFAULT 0,
		labels TEXT DEFAULT '[]',
		priority TEXT DEFAULT 'P3',
		priority_order INTEGER DEFAULT 99,
		category TEXT DEFAULT 'other'
	);

	CREATE TABLE IF NOT EXISTS related_prs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		issue_number INTEGER NOT NULL,
		pr_number INTEGER NOT NULL,
		pr_title TEXT NOT NULL,
		pr_html_url TEXT NOT NULL,
		FOREIGN KEY (issue_number) REFERENCES issues(number)
	);

	CREATE TABLE IF NOT EXISTS issue_tags (
		issue_number INTEGER PRIMARY KEY,
		tag TEXT NOT NULL DEFAULT 'none',
		updated_at TEXT NOT NULL,
		FOREIGN KEY (issue_number) REFERENCES issues(number)
	);

	CREATE TABLE IF NOT EXISTS meta (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_issues_priority ON issues(priority);
	CREATE INDEX IF NOT EXISTS idx_issues_category ON issues(category);
	CREATE INDEX IF NOT EXISTS idx_issues_updated ON issues(updated_at);
	`
	_, err := d.db.Exec(schema)
	return err
}

func (d *DB) UpsertIssue(issue *Issue) error {
	labelsJSON, _ := json.Marshal(issue.Labels)
	_, err := d.db.Exec(`
		INSERT INTO issues (id, number, title, html_url, body, user_login, user_avatar, created_at, updated_at, comments, labels, priority, priority_order, category)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(number) DO UPDATE SET
			title=excluded.title, html_url=excluded.html_url, body=excluded.body,
			user_login=excluded.user_login, user_avatar=excluded.user_avatar,
			updated_at=excluded.updated_at, comments=excluded.comments,
			labels=excluded.labels, priority=excluded.priority,
			priority_order=excluded.priority_order, category=excluded.category
	`, issue.ID, issue.Number, issue.Title, issue.HTMLURL, issue.Body,
		issue.UserLogin, issue.UserAvatar, issue.CreatedAt.Format(time.RFC3339),
		issue.UpdatedAt.Format(time.RFC3339), issue.Comments, string(labelsJSON),
		issue.Priority, issue.PriorityOrder, issue.Category)
	return err
}

func (d *DB) ClearRelatedPRs(issueNumber int) error {
	_, err := d.db.Exec("DELETE FROM related_prs WHERE issue_number = ?", issueNumber)
	return err
}

func (d *DB) InsertRelatedPR(issueNumber int, pr *RelatedPR) error {
	_, err := d.db.Exec(
		"INSERT INTO related_prs (issue_number, pr_number, pr_title, pr_html_url) VALUES (?, ?, ?, ?)",
		issueNumber, pr.Number, pr.Title, pr.HTMLURL)
	return err
}

func (d *DB) SetIssueTag(number int, tag string) error {
	if tag == TagNone {
		_, err := d.db.Exec("DELETE FROM issue_tags WHERE issue_number = ?", number)
		return err
	}
	_, err := d.db.Exec(`
		INSERT INTO issue_tags (issue_number, tag, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(issue_number) DO UPDATE SET tag=excluded.tag, updated_at=excluded.updated_at
	`, number, tag, time.Now().Format(time.RFC3339))
	return err
}

func (d *DB) GetIssueTag(number int) string {
	var tag string
	row := d.db.QueryRow("SELECT tag FROM issue_tags WHERE issue_number = ?", number)
	if row.Scan(&tag) != nil {
		return TagNone
	}
	return tag
}

func (d *DB) GetAllTags() (map[int]string, error) {
	rows, err := d.db.Query("SELECT issue_number, tag FROM issue_tags")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[int]string)
	for rows.Next() {
		var num int
		var tag string
		if err := rows.Scan(&num, &tag); err == nil {
			result[num] = tag
		}
	}
	return result, nil
}

func (d *DB) TagIssuesWithPRs(tag string) (int, error) {
	var totalPRs int
	d.db.QueryRow("SELECT COUNT(*) FROM related_prs").Scan(&totalPRs)
	var totalIssuesWithPRs int
	d.db.QueryRow("SELECT COUNT(DISTINCT issue_number) FROM related_prs").Scan(&totalIssuesWithPRs)
	var alreadyTagged int
	d.db.QueryRow("SELECT COUNT(DISTINCT r.issue_number) FROM related_prs r INNER JOIN issue_tags t ON r.issue_number = t.issue_number WHERE t.tag != ?", TagNone).Scan(&alreadyTagged)

	fmt.Printf("  数据库中 related_prs 总数: %d, 涉及 issue 数: %d, 已标记非 none: %d\n", totalPRs, totalIssuesWithPRs, alreadyTagged)

	rows, err := d.db.Query("SELECT DISTINCT issue_number FROM related_prs")
	if err != nil {
		return 0, err
	}
	var nums []int
	for rows.Next() {
		var num int
		if err := rows.Scan(&num); err == nil {
			nums = append(nums, num)
		}
	}
	rows.Close()

	now := time.Now().Format(time.RFC3339)
	count := 0
	for _, num := range nums {
		var currentTag string
		err := d.db.QueryRow("SELECT tag FROM issue_tags WHERE issue_number = ?", num).Scan(&currentTag)
		if err == sql.ErrNoRows || currentTag == TagNone {
			_, err := d.db.Exec(`
				INSERT INTO issue_tags (issue_number, tag, updated_at) VALUES (?, ?, ?)
				ON CONFLICT(issue_number) DO UPDATE SET tag=excluded.tag, updated_at=excluded.updated_at
			`, num, tag, now)
			if err == nil {
				count++
			}
		}
	}
	return count, nil
}

func (d *DB) GetStats() (*Stats, error) {
	s := &Stats{
		PriorityCounts: make(map[string]int),
		CategoryCounts: make(map[string]int),
	}

	row := d.db.QueryRow("SELECT COUNT(*) FROM issues")
	row.Scan(&s.TotalIssues)

	rows, err := d.db.Query("SELECT priority, COUNT(*) FROM issues GROUP BY priority")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var pri, cnt string
			if rows.Scan(&pri, &cnt) == nil {
				var n int
				fmt.Sscanf(cnt, "%d", &n)
				s.PriorityCounts[pri] = n
			}
		}
	}

	rows2, err := d.db.Query("SELECT category, COUNT(*) FROM issues GROUP BY category")
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var cat, cnt string
			if rows2.Scan(&cat, &cnt) == nil {
				var n int
				fmt.Sscanf(cnt, "%d", &n)
				s.CategoryCounts[cat] = n
			}
		}
	}

	// Count bug-labeled issues from labels JSON
	row = d.db.QueryRow(`SELECT COUNT(*) FROM issues WHERE labels LIKE '%"bug"%'`)
	row.Scan(&s.BugCount)

	// Count enhancement-labeled issues from labels JSON
	row = d.db.QueryRow(`SELECT COUNT(*) FROM issues WHERE labels LIKE '%"enhancement"%'`)
	row.Scan(&s.EnhancementCount)

	return s, nil
}

func (d *DB) QueryIssues(p FilterParams) (*PageResult, error) {
	conditions := []string{"1=1"}
	args := []interface{}{}

	if len(p.Priorities) > 0 {
		placeholders := make([]string, len(p.Priorities))
		for i, v := range p.Priorities {
			placeholders[i] = "?"
			args = append(args, v)
		}
		conditions = append(conditions, fmt.Sprintf("i.priority IN (%s)", strings.Join(placeholders, ",")))
	}
	if p.Category != "" {
		conditions = append(conditions, "i.category = ?")
		args = append(args, p.Category)
	}
	if p.Search != "" {
		conditions = append(conditions, "(i.title LIKE ? OR CAST(i.number AS TEXT) = ? OR i.user_login LIKE ?)")
		q := "%" + p.Search + "%"
		args = append(args, q, p.Search, q)
	}

	where := strings.Join(conditions, " AND ")

	orderBy := "i.priority_order ASC, i.number DESC"
	switch p.Sort {
	case "priority-desc":
		orderBy = "i.priority_order DESC, i.number DESC"
	case "updated-desc":
		orderBy = "i.updated_at DESC"
	case "created-desc":
		orderBy = "i.created_at DESC"
	case "comments-desc":
		orderBy = "i.comments DESC, i.priority_order ASC"
	}

	// Count total
	var total int
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM issues i WHERE %s", where)
	if err := d.db.QueryRow(countSQL, args...).Scan(&total); err != nil {
		return nil, err
	}

	// Fetch all issues (with tag), then filter in-memory for tag + pagination
	allSQL := fmt.Sprintf(`
		SELECT i.id, i.number, i.title, i.html_url, i.user_login, i.user_avatar,
			   i.created_at, i.updated_at, i.comments, i.labels, i.priority, i.priority_order, i.category,
			   COALESCE(t.tag, 'none') as tag
		FROM issues i
		LEFT JOIN issue_tags t ON i.number = t.issue_number
		WHERE %s
		ORDER BY %s
	`, where, orderBy)

	rows, err := d.db.Query(allSQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	allItems := []IssueDetail{}

	for rows.Next() {
		var id, number, comments int
		var title, htmlURL, userLogin, userAvatar, createdAt, updatedAt string
		var labelsJSON, priority, category, tag string

		var priorityOrder int
		if err := rows.Scan(&id, &number, &title, &htmlURL, &userLogin, &userAvatar,
			&createdAt, &updatedAt, &comments, &labelsJSON, &priority, &priorityOrder, &category, &tag); err != nil {
			continue
		}

		// Tag filter
		if p.Tag != "" && p.Tag != TagNone && tag != p.Tag {
			continue
		}

		ca, _ := time.Parse(time.RFC3339, createdAt)
		ua, _ := time.Parse(time.RFC3339, updatedAt)
		var labels []Label
		json.Unmarshal([]byte(labelsJSON), &labels)

		detail := IssueDetail{
			Issue: Issue{
				ID:         int64(id),
				Number:     number,
				Title:      title,
				HTMLURL:    htmlURL,
				UserLogin:  userLogin,
				UserAvatar: userAvatar,
				CreatedAt:  ca,
				UpdatedAt:  ua,
				Comments:   comments,
				Labels:     labels,
				Priority:   priority,
				Category:   category,
				Tag:        tag,
			},
		}
		allItems = append(allItems, detail)
	}

	// Apply "none" tag filter in-memory
	if p.Tag == TagNone {
		filtered := []IssueDetail{}
		for _, item := range allItems {
			if item.Tag == TagNone || item.Tag == "" {
				filtered = append(filtered, item)
			}
		}
		allItems = filtered
	}

	// Adjust total based on filtered count
	total = len(allItems)

	// Paginate
	pageCount := (total + p.PageSize - 1) / p.PageSize
	if pageCount < 1 {
		pageCount = 1
	}
	page := p.Page
	if page > pageCount {
		page = pageCount
	}
	start := (page - 1) * p.PageSize
	end := start + p.PageSize
	if start >= total {
		start = 0
		end = 0
	}
	if end > total {
		end = total
	}
	items := allItems[start:end]

	// Fetch related PRs
	if len(items) > 0 {
		prRows, _ := d.db.Query("SELECT issue_number, pr_number, pr_title, pr_html_url FROM related_prs")
		if prRows != nil {
			prMap := map[int][]RelatedPR{}
			for prRows.Next() {
				var issueNum, prNum int
				var prTitle, prURL string
				if prRows.Scan(&issueNum, &prNum, &prTitle, &prURL) == nil {
					prMap[issueNum] = append(prMap[issueNum], RelatedPR{Number: prNum, Title: prTitle, HTMLURL: prURL})
				}
			}
			prRows.Close()
			for i := range items {
				if prList, ok := prMap[items[i].Number]; ok {
					items[i].RelatedPRs = prList
				}
			}
		}
	}

	stats, _ := d.GetStats()
	return &PageResult{
		Items:     items,
		Total:     total,
		Page:      page,
		PageSize:  p.PageSize,
		PageCount: pageCount,
		Stats:     *stats,
	}, nil
}

func (d *DB) GetAllIssues() ([]Issue, error) {
	rows, err := d.db.Query(`
		SELECT i.id, i.number, i.title, i.html_url, i.user_login, i.user_avatar,
			   i.created_at, i.updated_at, i.comments, i.labels, i.priority, i.category,
			   COALESCE(t.tag, 'none')
		FROM issues i LEFT JOIN issue_tags t ON i.number = t.issue_number
		ORDER BY i.priority_order ASC, i.number DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	issues := []Issue{}
	for rows.Next() {
		var id, number, comments int
		var title, htmlURL, userLogin, userAvatar, createdAt, updatedAt string
		var labelsJSON, priority, category, tag string

		if rows.Scan(&id, &number, &title, &htmlURL, &userLogin, &userAvatar,
			&createdAt, &updatedAt, &comments, &labelsJSON, &priority, &category, &tag) != nil {
			continue
		}

		ca, _ := time.Parse(time.RFC3339, createdAt)
		ua, _ := time.Parse(time.RFC3339, updatedAt)
		var labels []Label
		json.Unmarshal([]byte(labelsJSON), &labels)

		issues = append(issues, Issue{
			ID:         int64(id),
			Number:     number,
			Title:      title,
			HTMLURL:    htmlURL,
			UserLogin:  userLogin,
			UserAvatar: userAvatar,
			CreatedAt:  ca,
			UpdatedAt:  ua,
			Comments:   comments,
			Labels:     labels,
			Priority:   priority,
			Category:   category,
			Tag:        tag,
		})
	}
	return issues, nil
}

func (d *DB) ReclassifyAll() (int, error) {
	rows, err := d.db.Query("SELECT number, title, COALESCE(body, ''), labels, COALESCE(updated_at, ''), comments FROM issues")
	if err != nil {
		return 0, err
	}

	type rec struct {
		number, comments int
		title, body, labelsJSON, updatedAt string
	}
	var records []rec
	for rows.Next() {
		var r rec
		if err := rows.Scan(&r.number, &r.title, &r.body, &r.labelsJSON, &r.updatedAt, &r.comments); err != nil {
			continue
		}
		records = append(records, r)
	}
	rows.Close()

	updated := 0
	for _, r := range records {
		var labels []Label
		json.Unmarshal([]byte(r.labelsJSON), &labels)
		labelNames := []string{}
		for _, l := range labels {
			labelNames = append(labelNames, strings.ToLower(l.Name))
		}

		isBug := containsSlice(labelNames, "bug")
		priority := determinePriority(labelNames, isBug, r.comments, r.updatedAt)
		category := determineCategory(labelNames, r.title, r.body)

		_, err := d.db.Exec(
			"UPDATE issues SET priority=?, priority_order=?, category=? WHERE number=?",
			priority, PriorityOrder[priority], category, r.number,
		)
		if err != nil {
			log.Printf("Reclassify #%d error: %v", r.number, err)
		} else {
			updated++
		}
	}
	log.Printf("ReclassifyAll: scanned=%d, updated=%d", len(records), updated)
	return updated, nil
}

func containsSlice(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func (d *DB) SetMeta(key, value string) error {
	_, err := d.db.Exec("INSERT INTO meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", key, value)
	return err
}

func (d *DB) GetMeta(key string) string {
	var val string
	d.db.QueryRow("SELECT value FROM meta WHERE key = ?", key).Scan(&val)
	return val
}

// Priority order map for sorting
var PriorityOrder = map[string]int{
	"P0": 0, "P1": 1, "P2": 2, "P3": 3,
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}
