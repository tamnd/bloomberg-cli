package bloomberg

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

const fakeRSS = `<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0">
  <channel>
    <title>Bloomberg - Top Headlines</title>
    <link>https://www.bloomberg.com</link>
    <description>Bloomberg top news</description>
    <item>
      <title>Fed Raises Rates by 25 Basis Points</title>
      <link>https://www.bloomberg.com/news/articles/2024-01-15/fed-raises-rates</link>
      <guid isPermaLink="true">https://www.bloomberg.com/news/articles/2024-01-15/fed-raises-rates</guid>
      <pubDate>Mon, 15 Jan 2024 10:00:00 +0000</pubDate>
      <description>The Federal Reserve raised interest rates.</description>
    </item>
    <item>
      <title>Apple Reports Record Earnings</title>
      <link>https://www.bloomberg.com/news/articles/2024-01-15/apple-earnings</link>
      <guid isPermaLink="true">https://www.bloomberg.com/news/articles/2024-01-15/apple-earnings</guid>
      <pubDate>Mon, 15 Jan 2024 11:00:00 +0000</pubDate>
      <description>Apple Inc. reported record quarterly earnings.</description>
    </item>
  </channel>
</rss>`

const fakeRSSNoPubDate = `<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0">
  <channel>
    <title>Bloomberg - Crypto</title>
    <link>https://www.bloomberg.com</link>
    <description>Bloomberg crypto news</description>
    <item>
      <title>Bitcoin Hits New High</title>
      <link>https://www.bloomberg.com/news/articles/2024-01-15/bitcoin-high</link>
      <guid isPermaLink="true">https://www.bloomberg.com/news/articles/2024-01-15/bitcoin-high</guid>
      <pubDate></pubDate>
      <description>Bitcoin reached a new all-time high.</description>
    </item>
  </channel>
</rss>`

func newTestClient(srv *httptest.Server) *Client {
	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	return NewClient(cfg)
}

func TestFeedTop(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(fakeRSS))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	articles, err := c.Feed(context.Background(), "top", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 2 {
		t.Fatalf("want 2 articles, got %d", len(articles))
	}
	if articles[0].Rank != 1 {
		t.Errorf("rank = %d, want 1", articles[0].Rank)
	}
	if articles[0].Title != "Fed Raises Rates by 25 Basis Points" {
		t.Errorf("title = %q", articles[0].Title)
	}
	if articles[0].URL != "https://www.bloomberg.com/news/articles/2024-01-15/fed-raises-rates" {
		t.Errorf("url = %q", articles[0].URL)
	}
	if articles[1].Rank != 2 {
		t.Errorf("second article rank = %d, want 2", articles[1].Rank)
	}
}

func TestFeedLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(fakeRSS))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	articles, err := c.Feed(context.Background(), "top", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 1 {
		t.Fatalf("want 1 article with limit=1, got %d", len(articles))
	}
}

func TestFeedUnknownSection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(fakeRSS))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.Feed(context.Background(), "nonexistent", 0)
	if !errors.Is(err, ErrUnknownSection) {
		t.Fatalf("got %v, want ErrUnknownSection", err)
	}
}

func TestFeedRetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(fakeRSS))
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 5
	c := NewClient(cfg)

	articles, err := c.Feed(context.Background(), "top", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) == 0 {
		t.Error("want articles after retries, got none")
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
}

func TestFeedPubDateParsing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(fakeRSSNoPubDate))
	}))
	defer srv.Close()

	// Feed with empty pubDate should produce empty Published field
	c := newTestClient(srv)
	articles, err := c.Feed(context.Background(), "crypto", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 1 {
		t.Fatalf("want 1 article, got %d", len(articles))
	}
	if articles[0].Published != "" {
		t.Errorf("expected empty Published for missing pubDate, got %q", articles[0].Published)
	}

	// Direct helper test with valid RFC1123Z value
	got := parsePubDate("Mon, 15 Jan 2024 10:00:00 +0000")
	if got != "2024-01-15 10:00" {
		t.Errorf("parsePubDate = %q, want %q", got, "2024-01-15 10:00")
	}
}

func TestSections(t *testing.T) {
	cfg := DefaultConfig()
	c := NewClient(cfg)
	sections := c.Sections()
	if len(sections) != 8 {
		t.Fatalf("want 8 sections, got %d", len(sections))
	}
	for i, s := range sections {
		if s.Name == "" {
			t.Errorf("section %d has empty Name", i)
		}
		if s.URL == "" {
			t.Errorf("section %d has empty URL", i)
		}
		if s.Feed == "" {
			t.Errorf("section %d has empty Feed slug", i)
		}
		if s.Rank != i+1 {
			t.Errorf("section %d rank = %d, want %d", i, s.Rank, i+1)
		}
	}
}
