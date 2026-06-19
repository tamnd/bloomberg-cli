// Package bloomberg is the library behind the bloom command: the HTTP client,
// request shaping, and typed data models for Bloomberg public RSS feeds.
//
// All Bloomberg feeds share one base URL; the section slug selects the path.
// The client sets a real User-Agent, paces requests, and retries 429/5xx
// with exponential back-off. No API key is required.
package bloomberg

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const defaultBaseURL = "https://feeds.bloomberg.com"

// DefaultUserAgent identifies the client to Bloomberg.
const DefaultUserAgent = "bloom/dev (+https://github.com/tamnd/bloomberg-cli)"

// ErrUnknownSection is returned when the caller requests a section slug that
// is not in the registry.
var ErrUnknownSection = errors.New("unknown section")

// sectionEntry maps a slug to a display name, feed path, and description.
type sectionEntry struct {
	Slug string
	Name string
	Path string
	Desc string
}

// sectionRegistry is the ordered list of Bloomberg RSS feed sections.
var sectionRegistry = []sectionEntry{
	{Slug: "top", Name: "Top Headlines", Path: "/news.rss", Desc: "Front-page news across all sections"},
	{Slug: "markets", Name: "Markets", Path: "/markets/news.rss", Desc: "Equities, bonds, currencies, commodities"},
	{Slug: "technology", Name: "Technology", Path: "/technology/news.rss", Desc: "Tech companies, products, regulation"},
	{Slug: "politics", Name: "Politics", Path: "/politics/news.rss", Desc: "Government, elections, policy"},
	{Slug: "business", Name: "Business", Path: "/business/news.rss", Desc: "Corporate news, earnings, M&A"},
	{Slug: "economics", Name: "Economics", Path: "/economics/news.rss", Desc: "Macro data, central banks, trade"},
	{Slug: "personal-finance", Name: "Personal Finance", Path: "/wealth/news.rss", Desc: "Investing, retirement, taxes"},
	{Slug: "crypto", Name: "Crypto", Path: "/crypto/news.rss", Desc: "Cryptocurrencies, digital assets, DeFi"},
}

// Config holds constructor parameters for Client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   defaultBaseURL,
		UserAgent: DefaultUserAgent,
		Rate:      500 * time.Millisecond,
		Retries:   3,
		Timeout:   30 * time.Second,
	}
}

// Client talks to Bloomberg RSS feeds over HTTP.
type Client struct {
	httpClient *http.Client
	baseURL    string
	userAgent  string
	rate       time.Duration
	retries    int
	mu         sync.Mutex
	last       time.Time
}

// NewClient returns a Client configured from cfg.
func NewClient(cfg Config) *Client {
	base := cfg.BaseURL
	if base == "" {
		base = defaultBaseURL
	}
	return &Client{
		httpClient: &http.Client{Timeout: cfg.Timeout},
		baseURL:    base,
		userAgent:  cfg.UserAgent,
		rate:       cfg.Rate,
		retries:    cfg.Retries,
	}
}

// Feed fetches articles from the named section's RSS feed.
// section is a slug key such as "top", "markets", or "crypto".
// limit <= 0 returns all items in the feed.
func (c *Client) Feed(ctx context.Context, section string, limit int) ([]Article, error) {
	path, err := sectionPath(section)
	if err != nil {
		return nil, err
	}
	rawURL := c.baseURL + path
	body, err := c.get(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	var doc rssDoc
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parse rss %s: %w", rawURL, err)
	}
	items := doc.Channel.Items
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	out := make([]Article, len(items))
	for i, it := range items {
		out[i] = itemToArticle(it, i+1, section)
	}
	return out, nil
}

// Sections returns the ordered static registry of all known feed sections.
func (c *Client) Sections() []Section {
	out := make([]Section, len(sectionRegistry))
	for i, e := range sectionRegistry {
		out[i] = Section{
			Rank:        i + 1,
			Slug:        e.Slug,
			Name:        e.Name,
			Description: e.Desc,
			URL:         c.baseURL + e.Path,
		}
	}
	return out
}

// sectionPath returns the URL path for slug, or ErrUnknownSection.
func sectionPath(slug string) (string, error) {
	for _, e := range sectionRegistry {
		if e.Slug == slug {
			return e.Path, nil
		}
	}
	return "", fmt.Errorf("%w: %q", ErrUnknownSection, slug)
}

// get fetches a URL with pacing and retries.
func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
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
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")

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
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rate <= 0 {
		return
	}
	if wait := c.rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}
