package imomath

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes imomath as a kit Domain so a multi-domain host (ant)
// can enable it with a single blank import:
//
//	import _ "github.com/tamnd/imomath-cli/imomath"
//
// The same Domain builds the standalone imomath binary (see cli/root.go).
func init() { kit.Register(Domain{}) }

// Domain is the imomath driver. It carries no state.
type Domain struct{}

// Info describes the scheme, accepted hostnames, and the binary identity.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "imomath",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "imomath",
			Short:  "Browse imomath.com olympiad resources from the command line",
			Long: `imomath reads public imomath.com resources over plain HTTPS, shapes them
into clean records, and prints output that pipes into the rest of your tools.
No API key, nothing to run alongside it.

Commands:
  list       List resources for a topic (algebra, geometry, nt, combinatorics)
  problem    Show a specific resource page by ID
  search     Search across all topics by keyword
  export     Export all resources for a topic (or all topics)
  info       Show site information

imomath is an independent tool and is not affiliated with imomath.com.`,
			Site: Host,
			Repo: "https://github.com/tamnd/imomath-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{Name: "list", Group: "read", List: true,
		Summary: "List resources for a topic"}, listResources)

	kit.Handle(app, kit.OpMeta{Name: "problem", Group: "read", Single: true,
		Summary: "Show a resource page by ID",
		Args:    []kit.Arg{{Name: "id", Help: "page ID (the ?p= parameter value)"}}}, getProblem)

	kit.Handle(app, kit.OpMeta{Name: "search", Group: "read", List: true,
		Summary: "Search for resources by keyword",
		Args:    []kit.Arg{{Name: "query", Help: "search keyword"}}}, searchResources)

	kit.Handle(app, kit.OpMeta{Name: "export", Group: "read", List: true,
		Summary: "Export all resources for a topic (or all topics)"}, exportResources)

	kit.Handle(app, kit.OpMeta{Name: "info", Group: "read", Single: true,
		Summary: "Show site information"}, getInfo)
}

// newClient builds the Client from the kit config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.Timeout = cfg.Timeout
	}
	return NewClient(c), nil
}

// --- list ---

type listResourcesIn struct {
	Topic  string  `kit:"flag"        help:"filter by topic (algebra, geometry, nt, combinatorics)"`
	Limit  int     `kit:"flag,inherit" help:"max results (0 = no limit)"`
	Client *Client `kit:"inject"`
}

func listResources(ctx context.Context, in listResourcesIn, emit func(*Resource) error) error {
	items, err := in.Client.List(ctx, in.Topic, in.Limit)
	if err != nil {
		return mapErr(err)
	}
	for i := range items {
		if err := emit(&items[i]); err != nil {
			return err
		}
	}
	return nil
}

// --- problem ---

type getProblemIn struct {
	ID     string  `kit:"arg"    help:"page ID (the ?p= parameter value)"`
	Client *Client `kit:"inject"`
}

func getProblem(ctx context.Context, in getProblemIn, emit func(*Problem) error) error {
	if in.ID == "" {
		return errs.Usage("id is required")
	}
	p, err := in.Client.GetProblem(ctx, in.ID)
	if err != nil {
		return mapErr(err)
	}
	return emit(p)
}

// --- search ---

type searchResourcesIn struct {
	Query  string  `kit:"arg"         help:"search keyword"`
	Limit  int     `kit:"flag,inherit" help:"max results (0 = no limit)"`
	Client *Client `kit:"inject"`
}

func searchResources(ctx context.Context, in searchResourcesIn, emit func(*Resource) error) error {
	if in.Query == "" {
		return errs.Usage("query is required")
	}
	items, err := in.Client.Search(ctx, in.Query, in.Limit)
	if err != nil {
		return mapErr(err)
	}
	for i := range items {
		if err := emit(&items[i]); err != nil {
			return err
		}
	}
	return nil
}

// --- export ---

type exportResourcesIn struct {
	Topic  string  `kit:"flag"   help:"export resources for a specific topic (empty = all)"`
	Client *Client `kit:"inject"`
}

func exportResources(ctx context.Context, in exportResourcesIn, emit func(*Resource) error) error {
	items, err := in.Client.List(ctx, in.Topic, 0)
	if err != nil {
		return mapErr(err)
	}
	for i := range items {
		if err := emit(&items[i]); err != nil {
			return err
		}
	}
	return nil
}

// --- info ---

type getInfoIn struct {
	Client *Client `kit:"inject"`
}

func getInfo(ctx context.Context, in getInfoIn, emit func(*Info) error) error {
	info, err := in.Client.Info(ctx)
	if err != nil {
		return mapErr(err)
	}
	return emit(&info)
}

// Classify turns a URL or page ID into (type, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	input = strings.TrimSpace(input)
	if u, err2 := url.Parse(input); err2 == nil && (u.Scheme == "http" || u.Scheme == "https") {
		if p := u.Query().Get("p"); p != "" {
			return "problem", p, nil
		}
	}
	if input != "" {
		return "problem", input, nil
	}
	return "", "", errs.Usage("cannot classify %q: use a page ID or imomath.com URL", input)
}

// Locate is the inverse of Classify.
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "problem":
		return fmt.Sprintf("https://www.%s/index.cgi?p=%s", Host, url.QueryEscape(id)), nil
	default:
		return "", errs.Usage("imomath has no resource type %q", uriType)
	}
}

// mapErr converts errors to kit error kinds.
func mapErr(err error) error {
	return err
}
