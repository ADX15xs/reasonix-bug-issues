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
	port      = flag.Int("port", 8765, "HTTP server port")
	dbPath    = flag.String("db", "data/reasonix.db", "SQLite database path")
	serveOnly = flag.Bool("serve", false, "Only start HTTP server, skip fetching data")
	fetchOnly = flag.Bool("fetch", false, "Only fetch data, do not start server")
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

	fetchFunc := func() error {
		fmt.Println("Fetching issues from GitHub...")
		ghIssues, err := internal.FetchAllIssues("open")
		if err != nil {
			return fmt.Errorf("FetchAllIssues: %v", err)
		}
		fmt.Printf("  Fetched %d issues\n", len(ghIssues))

		fmt.Println("Fetching PRs from GitHub...")
		ghPRs, err := internal.FetchAllPRs("open")
		if err != nil {
			return fmt.Errorf("FetchAllPRs: %v", err)
		}
		fmt.Printf("  Fetched %d PRs\n", len(ghPRs))

		fmt.Println("Storing issues in database...")
		for _, ghIss := range ghIssues {
			iss := internal.ToInternalIssue(ghIss)
			if err := db.UpsertIssue(&iss); err != nil {
				log.Printf("UpsertIssue #%d error: %v", iss.Number, err)
			}
			prs := internal.FindRelatedPRs(iss.Number, iss.Title, ghPRs)
			db.ClearRelatedPRs(iss.Number)
			for _, pr := range prs {
				db.InsertRelatedPR(iss.Number, &pr)
			}
		}
		db.SetMeta("last_fetch", time.Now().Format(time.RFC3339))
		fmt.Println("Done.")
		return nil
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
