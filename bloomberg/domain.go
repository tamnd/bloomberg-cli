// domain.go wires the bloomberg library into the any-cli/kit framework.
//
// A multi-domain host imports this package with a blank import:
//
//	import _ "github.com/tamnd/bloomberg-cli/bloomberg"
//
// The init below registers the domain; the host then routes bloomberg:// URIs
// to the operations installed in Register. The standalone bloom binary and
// the host share one source of truth.
package bloomberg

import (
	"context"

	"github.com/tamnd/any-cli/kit"
)

// Host is the canonical hostname for Bloomberg, used for URI domain registration.
const Host = "www.bloomberg.com"

func init() { kit.Register(Domain{}) }

// Domain is the bloomberg kit driver. It carries no state; the per-run Client
// is built by the factory Register hands to kit.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "bloomberg",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "bloomberg",
			Short:  "A command line for Bloomberg news.",
			Long: `A command line for Bloomberg news.

bloomberg fetches public Bloomberg RSS feeds and shapes them into clean
records that pipe into the rest of your tools. No API key required.`,
			Site: "https://" + Host,
			Repo: "https://github.com/tamnd/bloomberg-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	// feeds: list available Bloomberg RSS feeds.
	kit.Handle(app, kit.OpMeta{Name: "feeds", Group: "news", List: true,
		URIType: "feed", Summary: "List available Bloomberg RSS feeds"}, feedsCmd)

	// news: fetch headlines with optional --section flag.
	kit.Handle(app, kit.OpMeta{Name: "news", Group: "news", List: true,
		URIType: "article", Summary: "Fetch Bloomberg news headlines"},
		newsCmd)

	// Per-section named ops: each maps directly to a known slug.
	registerSection(app, "markets", "Markets news and data")
	registerSection(app, "technology", "Technology news")
	registerSection(app, "politics", "Politics news")
	registerSection(app, "economics", "Economics news")
	registerSection(app, "business", "Business news")
	registerSection(app, "personal-finance", "Personal finance news")
	registerSection(app, "crypto", "Cryptocurrency news")
}

// registerSection installs one named op that fetches a fixed section.
func registerSection(app *kit.App, slug, summary string) {
	s := slug // capture for closure
	kit.Handle(app, kit.OpMeta{Name: s, Group: "sections", List: true,
		URIType: "article", Summary: summary},
		func(ctx context.Context, in sectionIn, emit func(*Article) error) error {
			in.Section = s
			return fetchSection(ctx, in, emit)
		})
}

// newClient builds the Bloomberg client from the kit Config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := NewClient(DefaultConfig())
	if cfg.UserAgent != "" {
		c.userAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.httpClient.Timeout = cfg.Timeout
	}
	return c, nil
}

// Feed is the record emitted by the feeds op.
type Feed struct {
	Name string `json:"name" kit:"id" table:"name"`
	URL  string `json:"url"          table:"url,url"`
}

// --- inputs ---

type feedsIn struct {
	Client *Client `kit:"inject"`
}

type newsIn struct {
	Section string  `kit:"flag" help:"feed section (default: top)"`
	Limit   int     `kit:"flag,inherit" help:"max results"`
	Client  *Client `kit:"inject"`
}

type sectionIn struct {
	Section string  // set by registerSection, not a flag
	Limit   int     `kit:"flag,inherit" help:"max results"`
	Client  *Client `kit:"inject"`
}

// --- handlers ---

func feedsCmd(_ context.Context, in feedsIn, emit func(*Feed) error) error {
	for _, s := range in.Client.Sections() {
		if err := emit(&Feed{Name: s.Feed, URL: s.URL}); err != nil {
			return err
		}
	}
	return nil
}

func newsCmd(ctx context.Context, in newsIn, emit func(*Article) error) error {
	section := in.Section
	if section == "" {
		section = "top"
	}
	articles, err := in.Client.Feed(ctx, section, in.Limit)
	if err != nil {
		return err
	}
	for i := range articles {
		if err := emit(&articles[i]); err != nil {
			return err
		}
	}
	return nil
}

func fetchSection(ctx context.Context, in sectionIn, emit func(*Article) error) error {
	articles, err := in.Client.Feed(ctx, in.Section, in.Limit)
	if err != nil {
		return err
	}
	for i := range articles {
		if err := emit(&articles[i]); err != nil {
			return err
		}
	}
	return nil
}
