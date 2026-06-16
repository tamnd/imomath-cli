package imomath

// Resource is a link from an imomath.com topic page.
// A resource could be a problem set, a collection of problems, or an article.
type Resource struct {
	Rank  int    `json:"rank"`
	ID    string `json:"id"`    // the ?p= CGI parameter value
	Title string `json:"title"`
	Topic string `json:"topic"` // Algebra, Combinatorics, Geometry, Number Theory
	URL   string `json:"url"`
	Type  string `json:"type"` // "problem" for now
}

// Problem is a single imomath.com page, with the full body snippet.
type Problem struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Topic   string `json:"topic"`
	Source  string `json:"source,omitempty"` // competition source if extractable
	URL     string `json:"url"`
	Snippet string `json:"snippet,omitempty"` // first few paragraphs of text
}

// Info holds aggregate information about the site.
type Info struct {
	Site    string   `json:"site"`
	Topics  []string `json:"topics"`
	BaseURL string   `json:"base_url"`
}
