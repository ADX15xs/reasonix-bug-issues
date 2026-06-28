package internal

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
)

type Handler struct {
	DB          *DB
	FetchFunc   func() error
}

func NewHandler(db *DB, fetchFunc func() error) *Handler {
	return &Handler{DB: db, FetchFunc: fetchFunc}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	path := r.URL.Path
	switch {
	case path == "/" || path == "/index.html":
		http.ServeFile(w, r, "templates/index.html")
	case strings.HasPrefix(path, "/static/"):
		http.ServeFile(w, r, path[1:]) // strip leading "/"
	case path == "/api/issues":
		h.handleIssues(w, r)
	case path == "/api/issues/tags":
		h.handleSetTag(w, r)
	case path == "/api/stats":
		h.handleStats(w, r)
	case path == "/api/tags/export":
		h.handleExportTags(w, r)
	case path == "/api/tags/import":
		h.handleImportTags(w, r)
	case path == "/refresh":
		h.handleRefresh(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) handleIssues(w http.ResponseWriter, r *http.Request) {
	p := FilterParams{
		Page:     1,
		PageSize: 50,
		Sort:     "priority-asc",
	}

	if q := r.URL.Query(); true {
		if p.Page, _ = strconv.Atoi(q.Get("page")); p.Page < 1 {
			p.Page = 1
		}
		if p.PageSize, _ = strconv.Atoi(q.Get("pageSize")); p.PageSize < 1 {
			p.PageSize = 50
		}
		if q.Get("search") != "" {
			p.Search = q.Get("search")
		}
		if q.Get("category") != "" {
			p.Category = q.Get("category")
		}
		if q.Get("tag") != "" {
			p.Tag = q.Get("tag")
		}
		if q.Get("sort") != "" {
			p.Sort = q.Get("sort")
		}
		if priorities := q.Get("priorities"); priorities != "" {
			p.Priorities = strings.Split(priorities, ",")
		}
	}

	result, err := h.DB.QueryIssues(p)
	if err != nil {
		log.Printf("QueryIssues error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *Handler) handleSetTag(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Number int    `json:"number"`
		Tag    string `json:"tag"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	validTags := map[string]bool{"none": true, "fixed": true, "following": true, "planned": true}
	if !validTags[req.Tag] {
		http.Error(w, "Invalid tag", http.StatusBadRequest)
		return
	}
	if err := h.DB.SetIssueTag(req.Number, req.Tag); err != nil {
		log.Printf("SetIssueTag error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"ok": "1"})
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.DB.GetStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (h *Handler) handleExportTags(w http.ResponseWriter, r *http.Request) {
	tags, err := h.DB.GetAllTags()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="reasonix-tags-%s.json"`, "export"))
	json.NewEncoder(w).Encode(tags)
}

func (h *Handler) handleImportTags(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var tags map[string]string
	if err := json.NewDecoder(r.Body).Decode(&tags); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	imported := 0
	for numStr, tag := range tags {
		num, err := strconv.Atoi(numStr)
		if err != nil {
			continue
		}
		if err := h.DB.SetIssueTag(num, tag); err == nil {
			imported++
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"imported": imported})
}

func (h *Handler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	// Re-fetch from GitHub in background
	go func() {
		if h.FetchFunc != nil {
			log.Println("[/refresh] Starting background fetch...")
			if err := h.FetchFunc(); err != nil {
				log.Printf("[/refresh] Fetch error: %v", err)
			} else {
				log.Println("[/refresh] Fetch complete")
			}
		}
	}()

	json.NewEncoder(w).Encode(map[string]string{"status": "fetching started"})
}
