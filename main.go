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
)

func main() {
	flag.Parse()

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
func sync(db *internal.DB) error {
	var since string
	if !*fullSync {
		since = db.GetMeta("last_fetch")
	}

	var mode string
	if since == "" {
		mode = "全量"
		fmt.Println("[全量同步]")
	} else {
		mode = "增量"
		if _, err := time.Parse(time.RFC3339, since); err != nil {
			mode = "全量"
			fmt.Printf("[全量同步] (上次时间戳无效: %s)\n", since)
		} else {
			fmt.Printf("[增量同步] since=%s\n", since)
		}
	}

	fmt.Printf("Fetching %s issues from GitHub...\n", mode)
	ghIssues, err := internal.FetchIssues(internal.FetchIssuesParams{
		State: "all",
		Since: since,
	})
	if err != nil {
		return fmt.Errorf("FetchIssues: %v", err)
	}

	fmt.Printf("  Fetched %d issues\n", len(ghIssues))

	fmt.Println("Fetching PRs from GitHub...")
	ghPRs, err := internal.FetchAllPRs("open")
	if err != nil {
		return fmt.Errorf("FetchAllPRs: %v", err)
	}
	fmt.Printf("  Fetched %d PRs\n", len(ghPRs))

	fmt.Println("Storing issues in database...")
	upserted := 0
	for _, ghIss := range ghIssues {
		iss := internal.ToInternalIssue(ghIss)
		if err := db.UpsertIssue(&iss); err != nil {
			log.Printf("UpsertIssue #%d error: %v", iss.Number, err)
			continue
		}
		upserted++

		prs := internal.FindRelatedPRs(iss.Number, iss.Title, ghPRs)
		db.ClearRelatedPRs(iss.Number)
		for _, pr := range prs {
			db.InsertRelatedPR(iss.Number, &pr)
		}
	}
	fmt.Printf("  Upserted %d issues\n", upserted)

	now := time.Now().Format(time.RFC3339)
	db.SetMeta("last_fetch", now)
	fmt.Printf("Done. last_fetch=%s\n", now)
	return nil
}
