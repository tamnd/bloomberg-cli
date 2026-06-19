package bloomberg

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeRSS is a minimal RSS 2.0 document matching Bloomberg's live wire format.
// Uses CDATA, dc:creator, media:content/thumbnail, and GMT pubDate.
const fakeRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss xmlns:dc="http://purl.org/dc/elements/1.1/"
     xmlns:media="http://search.yahoo.com/mrss/"
     version="2.0">
  <channel>
    <title><![CDATA[Bloomberg.com]]></title>
    <description><![CDATA[Bloomberg news]]></description>
    <item>
      <title><![CDATA[Markets Rally on Fed Comments]]></title>
      <link>https://www.bloomberg.com/news/articles/2024-01-15/markets-rally</link>
      <guid isPermaLink="true">https://www.bloomberg.com/news/articles/2024-01-15/markets-rally</guid>
      <dc:creator><![CDATA[Jane Smith]]></dc:creator>
      <pubDate>Mon, 15 Jan 2024 10:00:00 GMT</pubDate>
      <description><![CDATA[Stocks gained as Fed officials signaled patience.]]></description>
      <media:content url="https://assets.bwbx.io/img1.jpg" type="image/jpeg">
        <media:thumbnail url="https://assets.bwbx.io/img1.jpg"></media:thumbnail>
      </media:content>
    </item>
    <item>
      <title><![CDATA[Tech Earnings Beat Estimates]]></title>
      <link>https://www.bloomberg.com/news/articles/2024-01-15/tech-earnings</link>
      <guid isPermaLink="true">https://www.bloomberg.com/news/articles/2024-01-15/tech-earnings</guid>
      <pubDate>Mon, 15 Jan 2024 09:00:00 GMT</pubDate>
      <description><![CDATA[Major tech companies reported stronger than expected Q4 results.]]></description>
    </item>
  </channel>
</rss>`

func newTestClient(srv *httptest.Server) *Client {
	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 0
	return NewClient(cfg)
}

func serve(rss string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(rss))
	}))
}

// TestFeedParsesItems checks that two items are parsed with correct rank, title,
// published date, section, thumbnail, and URL.
func TestFeedParsesItems(t *testing.T) {
	srv := serve(fakeRSS)
	defer srv.Close()

	c := newTestClient(srv)
	articles, err := c.Feed(context.Background(), "top", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 2 {
		t.Fatalf("want 2 articles, got %d", len(articles))
	}
	a := articles[0]
	if a.Rank != 1 {
		t.Errorf("rank = %d, want 1", a.Rank)
	}
	if a.Title != "Markets Rally on Fed Comments" {
		t.Errorf("title = %q", a.Title)
	}
	if a.Published != "2024-01-15 10:00" {
		t.Errorf("published = %q, want %q", a.Published, "2024-01-15 10:00")
	}
	if a.Section != "top" {
		t.Errorf("section = %q, want top", a.Section)
	}
	if a.URL != "https://www.bloomberg.com/news/articles/2024-01-15/markets-rally" {
		t.Errorf("url = %q", a.URL)
	}
	if a.ThumbnailURL != "https://assets.bwbx.io/img1.jpg" {
		t.Errorf("thumbnail_url = %q", a.ThumbnailURL)
	}
	if articles[1].Rank != 2 {
		t.Errorf("second article rank = %d, want 2", articles[1].Rank)
	}
}

// TestFeedLimit verifies that limit=1 trims a two-item feed to one article.
func TestFeedLimit(t *testing.T) {
	srv := serve(fakeRSS)
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

// TestFeedLimitZero verifies that limit=0 returns all items.
func TestFeedLimitZero(t *testing.T) {
	srv := serve(fakeRSS)
	defer srv.Close()

	c := newTestClient(srv)
	articles, err := c.Feed(context.Background(), "top", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 2 {
		t.Fatalf("want 2 articles with limit=0, got %d", len(articles))
	}
}

// TestFeedUnknownSection verifies ErrUnknownSection is returned for bad slugs.
func TestFeedUnknownSection(t *testing.T) {
	srv := serve(fakeRSS)
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.Feed(context.Background(), "nonexistent", 0)
	if !errors.Is(err, ErrUnknownSection) {
		t.Fatalf("got %v, want ErrUnknownSection", err)
	}
}

// TestFeedRetriesOn503 verifies the client retries on 503 and succeeds.
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

// TestFeedEmptyPubDate checks that an empty pubDate yields Published=="".
func TestFeedEmptyPubDate(t *testing.T) {
	rss := `<?xml version="1.0"?><rss version="2.0"><channel>
	  <item>
	    <title>Test</title>
	    <link>https://example.com/a</link>
	    <guid>https://example.com/a</guid>
	    <pubDate></pubDate>
	    <description>desc</description>
	  </item>
	</channel></rss>`
	srv := serve(rss)
	defer srv.Close()

	c := newTestClient(srv)
	articles, err := c.Feed(context.Background(), "top", 0)
	if err != nil {
		t.Fatal(err)
	}
	if articles[0].Published != "" {
		t.Errorf("want empty Published for missing pubDate, got %q", articles[0].Published)
	}
}

// TestFeedHTMLEntities checks entity decoding in title and description.
func TestFeedHTMLEntities(t *testing.T) {
	rss := `<?xml version="1.0"?><rss version="2.0"><channel>
	  <item>
	    <title>S&amp;P 500 &lt;Rises&gt;</title>
	    <link>https://example.com/sp</link>
	    <guid>https://example.com/sp</guid>
	    <pubDate>Mon, 15 Jan 2024 10:00:00 GMT</pubDate>
	    <description>Index rose amid &quot;cautious optimism&quot; &amp; strong earnings.</description>
	  </item>
	</channel></rss>`
	srv := serve(rss)
	defer srv.Close()

	c := newTestClient(srv)
	articles, err := c.Feed(context.Background(), "top", 0)
	if err != nil {
		t.Fatal(err)
	}
	if articles[0].Title != "S&P 500 <Rises>" {
		t.Errorf("title = %q, want %q", articles[0].Title, "S&P 500 <Rises>")
	}
	if articles[0].Description != `Index rose amid "cautious optimism" & strong earnings.` {
		t.Errorf("description = %q", articles[0].Description)
	}
}

// TestFeedGUIDFallback checks that an empty <link> falls back to <guid>.
func TestFeedGUIDFallback(t *testing.T) {
	rss := `<?xml version="1.0"?><rss version="2.0"><channel>
	  <item>
	    <title>Audio Clip</title>
	    <link></link>
	    <guid isPermaLink="true">https://www.bloomberg.com/news/audio/2024-01-15/audio-clip</guid>
	    <pubDate>Mon, 15 Jan 2024 10:00:00 GMT</pubDate>
	    <description>A podcast episode.</description>
	  </item>
	</channel></rss>`
	srv := serve(rss)
	defer srv.Close()

	c := newTestClient(srv)
	articles, err := c.Feed(context.Background(), "top", 0)
	if err != nil {
		t.Fatal(err)
	}
	want := "https://www.bloomberg.com/news/audio/2024-01-15/audio-clip"
	if articles[0].URL != want {
		t.Errorf("url = %q, want %q", articles[0].URL, want)
	}
}

// TestFeedThumbnail checks that media:content/media:thumbnail url is captured.
func TestFeedThumbnail(t *testing.T) {
	srv := serve(fakeRSS)
	defer srv.Close()

	c := newTestClient(srv)
	articles, err := c.Feed(context.Background(), "top", 0)
	if err != nil {
		t.Fatal(err)
	}
	if articles[0].ThumbnailURL != "https://assets.bwbx.io/img1.jpg" {
		t.Errorf("thumbnail_url = %q, want %q", articles[0].ThumbnailURL, "https://assets.bwbx.io/img1.jpg")
	}
	// second item has no thumbnail
	if articles[1].ThumbnailURL != "" {
		t.Errorf("second article thumbnail_url = %q, want empty", articles[1].ThumbnailURL)
	}
}

// TestSectionsCount checks that Sections returns exactly 8 entries.
func TestSectionsCount(t *testing.T) {
	c := NewClient(DefaultConfig())
	sections := c.Sections()
	if len(sections) != 8 {
		t.Fatalf("want 8 sections, got %d", len(sections))
	}
	for i, s := range sections {
		if s.Slug == "" {
			t.Errorf("sections[%d].Slug is empty", i)
		}
		if s.Name == "" {
			t.Errorf("sections[%d].Name is empty", i)
		}
		if s.URL == "" {
			t.Errorf("sections[%d].URL is empty", i)
		}
	}
}

// TestSectionsRank checks that Rank is 1-based and sequential.
func TestSectionsRank(t *testing.T) {
	c := NewClient(DefaultConfig())
	sections := c.Sections()
	for i, s := range sections {
		if s.Rank != i+1 {
			t.Errorf("sections[%d].Rank = %d, want %d", i, s.Rank, i+1)
		}
	}
}

// TestFeedRFC1123Date verifies RFC1123 with GMT suffix parses correctly.
func TestFeedRFC1123Date(t *testing.T) {
	got := parsePubDate("Mon, 15 Jun 2026 02:18:04 GMT")
	want := "2026-06-15 02:18"
	if got != want {
		t.Errorf("parsePubDate(GMT) = %q, want %q", got, want)
	}
}

// TestFeed404Terminal verifies that a 404 response is not retried.
func TestFeed404Terminal(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 3
	c := NewClient(cfg)

	_, err := c.Feed(context.Background(), "top", 0)
	if err == nil {
		t.Fatal("want error on 404, got nil")
	}
	if hits != 1 {
		t.Errorf("server hits = %d, want 1 (no retry on 404)", hits)
	}
}
