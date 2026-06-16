package imomath_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tamnd/imomath-cli/imomath"
)

const algebraHTML = `<!DOCTYPE html>
<html>
<head><title>imomath - Algebra</title></head>
<body>
<h1>Algebra Problems</h1>
<ul>
<li><a href="/index.cgi?p=algebra_problems1">Algebra Problem Set 1</a></li>
<li><a href="/index.cgi?p=algebra_problems2">Algebra Problem Set 2</a></li>
<li><a href="/index.cgi?p=algebra_basics">Algebra Basics</a></li>
<li><a href="/index.cgi?p=algebra">Back to Algebra</a></li>
</ul>
</body>
</html>`

const problemHTML = `<!DOCTYPE html>
<html>
<head><title>imomath - Algebra Problem Set 1</title></head>
<body>
<h1>Algebra Problem Set 1</h1>
<p>This is the first set of algebra problems for olympiad training.</p>
<p>These problems cover polynomial inequalities and functional equations.</p>
</body>
</html>`

const indexHTML = `<!DOCTYPE html>
<html>
<head><title>imomath.com - Olympiad Math</title></head>
<body>
<h1>imomath.com</h1>
<p>Welcome to imomath.com, your resource for olympiad mathematics.</p>
</body>
</html>`

func newTestClient(ts *httptest.Server) *imomath.Client {
	cfg := imomath.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	return imomath.NewClient(cfg)
}

func TestListAlgebra(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Query().Get("p")
		if p == "algebra" || r.URL.Path == "/index.cgi" && p == "algebra" {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(algebraHTML))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	items, err := c.List(context.Background(), "algebra", 0)
	if err != nil {
		t.Fatal(err)
	}
	// Should have 3 links (algebra_problems1, algebra_problems2, algebra_basics)
	// Not "algebra" itself (filtered as navigation).
	if len(items) != 3 {
		t.Errorf("got %d items, want 3; items: %+v", len(items), items)
	}
	if items[0].ID != "algebra_problems1" {
		t.Errorf("items[0].ID = %q, want algebra_problems1", items[0].ID)
	}
	if items[0].Topic != "Algebra" {
		t.Errorf("items[0].Topic = %q, want Algebra", items[0].Topic)
	}
	if items[0].Rank != 1 {
		t.Errorf("items[0].Rank = %d, want 1", items[0].Rank)
	}
}

func TestListWithLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(algebraHTML))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	items, err := c.List(context.Background(), "algebra", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Errorf("got %d items, want 1", len(items))
	}
}

func TestListUnknownTopic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.List(context.Background(), "bogus", 0)
	if err == nil {
		t.Fatal("expected error for unknown topic")
	}
}

func TestGetProblem(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(problemHTML))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	p, err := c.GetProblem(context.Background(), "algebra_problems1")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "algebra_problems1" {
		t.Errorf("ID = %q, want algebra_problems1", p.ID)
	}
	if !strings.Contains(p.Title, "Algebra Problem Set 1") {
		t.Errorf("Title = %q, should contain 'Algebra Problem Set 1'", p.Title)
	}
	if p.Snippet == "" {
		t.Error("Snippet should not be empty")
	}
	if !strings.Contains(p.URL, "algebra_problems1") {
		t.Errorf("URL = %q, should contain algebra_problems1", p.URL)
	}
}

func TestSearch_found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(algebraHTML))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	// Search across all 4 topics; each will return the algebraHTML.
	items, err := c.Search(context.Background(), "basics", 0)
	if err != nil {
		t.Fatal(err)
	}
	// "basics" should match "algebra_basics"; deduplicated across topics.
	found := false
	for _, r := range items {
		if strings.Contains(r.ID, "basics") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected to find a 'basics' resource; got: %+v", items)
	}
}

func TestSearch_notFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(algebraHTML))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	items, err := c.Search(context.Background(), "zzznomatch999", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(indexHTML))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	info, err := c.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.BaseURL == "" {
		t.Error("BaseURL should not be empty")
	}
	if len(info.Topics) == 0 {
		t.Error("Topics should not be empty")
	}
}

func TestGetSendsUserAgent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(indexHTML))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetRetriesOn503(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(indexHTML))
	}))
	defer srv.Close()

	cfg := imomath.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 3
	c := imomath.NewClient(cfg)

	info, err := c.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.BaseURL == "" {
		t.Error("BaseURL should not be empty after retry")
	}
}
