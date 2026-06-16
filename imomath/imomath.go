// Package imomath is the library behind the imomath command line:
// the HTTP client, request shaping, and typed data models for imomath.com.
//
// imomath.com uses a CGI-based architecture where pages are addressed by
// ?p=<page> query parameters. The client fetches HTML pages and extracts
// links, titles, and snippets from the response.
package imomath

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// DefaultUserAgent identifies the client to imomath.com.
const DefaultUserAgent = "imomath-cli/dev (+https://github.com/tamnd/imomath-cli)"

// Host is the imomath.com hostname.
const Host = "imomath.com"

// BaseURL is the root every request is built from.
const BaseURL = "https://www." + Host

// topicPages maps the short topic name to the CGI page parameter.
var topicPages = map[string]string{
	"algebra":       "algebra",
	"geometry":      "geometry",
	"nt":            "nt",
	"combinatorics": "combinatorics",
	"number-theory": "nt",
	"combo":         "combinatorics",
}

// validTopics is the ordered list of standard topic names.
var validTopics = []string{"algebra", "geometry", "nt", "combinatorics"}

// Config holds constructor parameters for the Client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Timeout   time.Duration
	Retries   int
}

// DefaultConfig returns sensible production defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   BaseURL,
		UserAgent: DefaultUserAgent,
		Rate:      500 * time.Millisecond,
		Timeout:   30 * time.Second,
		Retries:   3,
	}
}

// Client talks to imomath.com over HTTP.
type Client struct {
	cfg        Config
	httpClient *http.Client
	last       time.Time
}

// NewClient returns a Client ready to use.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: cfg.Timeout},
	}
}

// pageURL builds the CGI URL for a given page parameter.
func (c *Client) pageURL(page string) string {
	if page == "" || page == "index" {
		return c.cfg.BaseURL + "/index.cgi"
	}
	return c.cfg.BaseURL + "/index.cgi?p=" + url.QueryEscape(page)
}

// urlToID converts an href to a short page ID (the ?p= value).
func urlToID(href string) string {
	if !strings.Contains(href, "?") {
		return ""
	}
	u, err := url.Parse(href)
	if err != nil {
		return ""
	}
	return u.Query().Get("p")
}

// isProblemLink returns true if the link looks like a problem/resource page.
func isProblemLink(href string) bool {
	if href == "" || href == "/" {
		return false
	}
	if strings.HasPrefix(href, "http") && !strings.Contains(href, "imomath") {
		return false
	}
	id := urlToID(href)
	if id == "" {
		return false
	}
	// Skip the known top-level navigation pages.
	switch id {
	case "algebra", "geometry", "nt", "combinatorics", "about", "contact", "index":
		return false
	}
	return true
}

// resolveURL ensures a relative href becomes an absolute URL.
func (c *Client) resolveURL(href string) string {
	if strings.HasPrefix(href, "http") {
		return href
	}
	if strings.HasPrefix(href, "/") {
		base := c.cfg.BaseURL
		if i := strings.Index(base[8:], "/"); i >= 0 {
			base = base[:8+i]
		}
		return base + href
	}
	return c.cfg.BaseURL + "/" + strings.TrimPrefix(href, "/")
}

// List returns resources for the given topic (or all topics if topic is "").
func (c *Client) List(ctx context.Context, topic string, limit int) ([]Resource, error) {
	var topics []string
	if topic == "" {
		topics = validTopics
	} else {
		page, ok := topicPages[strings.ToLower(topic)]
		if !ok {
			return nil, fmt.Errorf("unknown topic %q; valid topics: %s", topic, strings.Join(validTopics, ", "))
		}
		topics = []string{page}
	}

	var result []Resource
	seen := map[string]bool{}
	rank := 0
	for _, t := range topics {
		body, err := c.get(ctx, c.pageURL(t))
		if err != nil {
			return nil, err
		}
		links := c.extractLinks(body, t)
		for _, r := range links {
			if seen[r.ID] {
				continue
			}
			seen[r.ID] = true
			rank++
			r.Rank = rank
			result = append(result, r)
			if limit > 0 && len(result) >= limit {
				return result, nil
			}
		}
	}
	return result, nil
}

// GetProblem fetches a problem/resource page by its page ID.
func (c *Client) GetProblem(ctx context.Context, id string) (*Problem, error) {
	pageURL := c.pageURL(id)
	body, err := c.get(ctx, pageURL)
	if err != nil {
		return nil, err
	}

	title := extractTitle(body)
	snippet := extractSnippet(body)

	// Determine topic from page content (look for navigation links).
	topic := guessTopic(id, body)

	return &Problem{
		ID:      id,
		Title:   title,
		Topic:   topic,
		URL:     pageURL,
		Snippet: snippet,
	}, nil
}

// Search fetches resources across all topics and filters by keyword.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]Resource, error) {
	all, err := c.List(ctx, "", 0)
	if err != nil {
		return nil, err
	}
	query = strings.ToLower(query)
	var result []Resource
	rank := 0
	for _, r := range all {
		if strings.Contains(strings.ToLower(r.Title), query) ||
			strings.Contains(strings.ToLower(r.ID), query) ||
			strings.Contains(strings.ToLower(r.Topic), query) {
			rank++
			r.Rank = rank
			result = append(result, r)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

// Info returns aggregate information about the site.
func (c *Client) Info(ctx context.Context) (Info, error) {
	body, err := c.get(ctx, c.cfg.BaseURL+"/index.cgi")
	if err != nil {
		return Info{}, err
	}
	title := extractTitle(body)
	return Info{
		Site:    title,
		Topics:  validTopics,
		BaseURL: c.cfg.BaseURL,
	}, nil
}

// extractLinks parses the HTML body and returns resources linked from a topic page.
func (c *Client) extractLinks(body []byte, topic string) []Resource {
	var resources []Resource
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		// Fall back to regex if HTML parsing fails.
		return c.extractLinksRegex(body, topic)
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					href := attr.Val
					if isProblemLink(href) {
						id := urlToID(href)
						text := nodeText(n)
						if text == "" {
							text = id
						}
						resources = append(resources, Resource{
							ID:    id,
							Title: text,
							Topic: topicDisplayName(topic),
							URL:   c.resolveURL(href),
							Type:  "problem",
						})
					}
					break
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return resources
}

// extractLinksRegex is a fallback link extractor using regex.
func (c *Client) extractLinksRegex(body []byte, topic string) []Resource {
	hrefRE := regexp.MustCompile(`href="([^"]+)"`)
	textRE := regexp.MustCompile(`>([^<]+)<`)
	var resources []Resource
	for _, m := range hrefRE.FindAllSubmatch(body, -1) {
		href := string(m[1])
		if !isProblemLink(href) {
			continue
		}
		id := urlToID(href)
		title := id
		if tm := textRE.Find(m[0]); tm != nil {
			title = strings.TrimSpace(string(tm[1 : len(tm)-1]))
		}
		resources = append(resources, Resource{
			ID:    id,
			Title: title,
			Topic: topicDisplayName(topic),
			URL:   c.resolveURL(href),
			Type:  "problem",
		})
	}
	return resources
}

// extractTitle extracts the page title from HTML.
func extractTitle(body []byte) string {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return ""
	}
	var title string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "title" {
			title = nodeText(n)
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return strings.TrimSpace(title)
}

// extractSnippet extracts a text snippet from the page body.
func extractSnippet(body []byte) string {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		// Fallback: strip tags.
		tagRE := regexp.MustCompile(`<[^>]+>`)
		s := strings.Join(strings.Fields(tagRE.ReplaceAllString(string(body), " ")), " ")
		if len(s) > 300 {
			s = s[:300]
		}
		return s
	}
	// Collect text from <p> elements.
	var parts []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "p" {
			text := strings.TrimSpace(nodeText(n))
			if len(text) > 20 {
				parts = append(parts, text)
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	if len(parts) == 0 {
		return ""
	}
	snippet := strings.Join(parts[:min(3, len(parts))], " ")
	if len(snippet) > 300 {
		snippet = snippet[:300]
	}
	return snippet
}

// guessTopic tries to determine the topic of a problem from its ID or body.
func guessTopic(id string, body []byte) string {
	id = strings.ToLower(id)
	for prefix, topic := range map[string]string{
		"alg": "Algebra",
		"geo": "Geometry",
		"nt":  "Number Theory",
		"com": "Combinatorics",
	} {
		if strings.HasPrefix(id, prefix) {
			return topic
		}
	}
	// Check body for topic links.
	for _, t := range validTopics {
		if bytes.Contains(body, []byte(`?p=`+t)) {
			return topicDisplayName(t)
		}
	}
	return ""
}

// topicDisplayName returns the human-readable topic name.
func topicDisplayName(page string) string {
	switch page {
	case "algebra":
		return "Algebra"
	case "geometry":
		return "Geometry"
	case "nt":
		return "Number Theory"
	case "combinatorics":
		return "Combinatorics"
	default:
		return page
	}
}

// nodeText returns all text content of an HTML node and its descendants.
func nodeText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		sb.WriteString(nodeText(child))
	}
	return sb.String()
}

// get fetches a URL and returns the body bytes with pacing and retry.
func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

// pace enforces the inter-request rate limit.
func (c *Client) pace() {
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		return 5 * time.Second
	}
	return d
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
