package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"reasonix-bug-report/internal"
)

var (
	port       = flag.Int("port", 8765, "HTTP server port")
	dbPath     = flag.String("db", "data/reasonix.db", "SQLite database path")
	serveOnly  = flag.Bool("serve", false, "Only start HTTP server, skip fetching data")
	fetchOnly  = flag.Bool("fetch", false, "Only fetch data, do not start server")
	fullSync   = flag.Bool("full", false, "Force full sync (ignore last_fetch)")
	reclassify = flag.Bool("reclassify", false, "Re-classify all existing issues in DB using current logic")
	tagPRs     = flag.Bool("tag-prs", false, "Tag all issues with related PRs as 'following' (已有人跟进)")
	stateFlag  = flag.String("state", "open", "Issue state to fetch: open, closed, all")
)

func main() {
	flag.Parse()

	internal.LoadDotenv()

	token := os.Getenv("GITHUB_TOKEN")
	if token != "" {
		fmt.Println("[GitHub Token] 已检测到 GITHUB_TOKEN，API 限额提升至 5000 次/小时")
	} else {
		fmt.Println("[GitHub Token] 未设置 GITHUB_TOKEN，使用未认证模式（60 次/小时）")
		fmt.Println("  提示：创建 .env 文件并设置 GITHUB_TOKEN 可大幅提高 API 限额")
	}

	if err := os.MkdirAll("data", 0755); err != nil {
		log.Fatalf("Create data dir: %v", err)
	}

	db, err := internal.OpenDB(*dbPath)
	if err != nil {
		log.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	if *reclassify {
		fmt.Println("[重分类] 正在重新分类所有 issue...")
		n, err := db.ReclassifyAll()
		if err != nil {
			log.Fatalf("Reclassify error: %v", err)
		}
		fmt.Printf("  已更新 %d 条 issue\n", n)
	}

	if *tagPRs {
		fmt.Println("[标记 PR 关联 issue] 正在将有关联 PR 的 issue 标记为已有人跟进...")
		n, err := db.TagIssuesWithPRs("following")
		if err != nil {
			log.Fatalf("TagPRs error: %v", err)
		}
		fmt.Printf("  已标记 %d 条 issue\n", n)
	}

	standalone := *tagPRs && !*serveOnly && !*fetchOnly
	if standalone {
		return
	}

	fetchFunc := func() error {
		return sync(db)
	}

	if !*serveOnly {
		if err := fetchFunc(); err != nil {
			log.Fatalf("Fetch error: %v", err)
		}
	}

	if !*fetchOnly {
		h := internal.NewHandler(db, fetchFunc)
		mux := http.NewServeMux()
		mux.HandleFunc("/", h.ServeHTTP)

		addr := fmt.Sprintf(":%d", *port)
		fmt.Printf("\nServer running at http://localhost:%d\n", *port)
		fmt.Printf("Open: http://localhost:%d/\n", *port)

		srv := &http.Server{Addr: addr, Handler: mux}
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}
}

// sync performs a full or incremental sync based on last_fetch timestamp.
//
// To detect closures, we always fetch with state=all (+ since for incremental).
// Otherwise state=open + since would never return issues that just got closed,
// and they would accumulate in the DB forever. The --state flag now controls
// which state is *kept* in the DB: issues whose state does not match are pruned
// after the upsert. A full sync additionally drops issues that no longer exist
// on GitHub.
func sync(db *internal.DB) error {
	var since string
	if !*fullSync {
		since = db.GetMeta("last_fetch")
	}

	isFullSync := since == ""
	if _, err := time.Parse(time.RFC3339, since); err != nil {
		isFullSync = true
		fmt.Printf("[全量同步] (上次时间戳无效: %s)\n", since)
	} else if isFullSync {
		fmt.Println("[全量同步]")
	} else {
		fmt.Printf("[增量同步] since=%s\n", since)
	}

	fmt.Printf("Fetching issues+PRs from GitHub (fetch state=all, keep state=%s)...\n", *stateFlag)
	ghIssues, ghPRs, err := internal.FetchIssuesAndPRs(internal.FetchIssuesParams{
		State: "all",
		Since: since,
	})
	if err != nil {
		return fmt.Errorf("FetchIssuesAndPRs: %v", err)
	}

	fmt.Printf("  Fetched %d issues, %d PRs (single endpoint, split client-side)\n", len(ghIssues), len(ghPRs))

	fmt.Println("Storing issues in database...")
	upserted := 0
	fetchedNumbers := make([]int, 0, len(ghIssues))
	for _, ghIss := range ghIssues {
		iss := internal.ToInternalIssue(ghIss)
		fetchedNumbers = append(fetchedNumbers, iss.Number)
		if err := db.UpsertIssue(&iss); err != nil {
			log.Printf("UpsertIssue #%d error: %v", iss.Number, err)
			continue
		}
		upserted++

		// Only link PRs for issues we are going to keep; closed issues (when
		// state=open) will be pruned below, so skip the work.
		if *stateFlag == "all" || iss.State == *stateFlag {
			prs := internal.FindRelatedPRs(iss.Number, iss.Title, ghPRs)
			db.ClearRelatedPRs(iss.Number)
			for _, pr := range prs {
				db.InsertRelatedPR(iss.Number, &pr)
			}
		}
	}
	fmt.Printf("  Upserted %d issues\n", upserted)

	// Prune issues whose state does not match the desired --state.
	if *stateFlag != "all" {
		n, err := db.DeleteIssuesNotInState(*stateFlag)
		if err != nil {
			log.Printf("DeleteIssuesNotInState error: %v", err)
		} else if n > 0 {
			fmt.Printf("  Pruned %d issues not in state=%s (e.g. closed)\n", n, *stateFlag)
		}
	}

	// On a full sync, also drop issues that no longer exist on GitHub.
	if isFullSync {
		n, err := db.DeleteIssuesNotInNumbers(fetchedNumbers)
		if err != nil {
			log.Printf("DeleteIssuesNotInNumbers error: %v", err)
		} else if n > 0 {
			fmt.Printf("  Pruned %d issues no longer present on GitHub\n", n)
		}
	}

	// Tidy up any related_prs rows left dangling by the deletions above.
	if n, err := db.CleanupOrphanedPRs(); err != nil {
		log.Printf("CleanupOrphanedPRs error: %v", err)
	} else if n > 0 {
		fmt.Printf("  Cleaned %d orphaned related_prs rows\n", n)
	}

	now := time.Now().Format(time.RFC3339)
	db.SetMeta("last_fetch", now)
	fmt.Printf("Done. last_fetch=%s\n", now)
	return nil
}
