package bloomberg

import (
	"encoding/xml"
	"html"
	"strings"
	"time"
)

// Article is the record emitted for each Bloomberg news item.
type Article struct {
	Rank        int    `json:"rank"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Published   string `json:"published"`
	URL         string `json:"url"`
}

// Section is the record emitted by the sections command.
type Section struct {
	Rank int    `json:"rank"`
	Name string `json:"name"`
	Feed string `json:"feed"`
	URL  string `json:"url"`
}

// ─── wire types ─────────────────────────────────────────────────────────────

type rssDoc struct {
	XMLName xml.Name  `xml:"rss"`
	Items   []rssItem `xml:"channel>item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	PubDate     string `xml:"pubDate"`
	Description string `xml:"description"`
}

// ─── conversion helpers ──────────────────────────────────────────────────────

func itemToArticle(it rssItem, rank int) Article {
	u := strings.TrimSpace(it.Link)
	if u == "" {
		u = strings.TrimSpace(it.GUID)
	}
	return Article{
		Rank:        rank,
		Title:       html.UnescapeString(strings.TrimSpace(it.Title)),
		Description: html.UnescapeString(strings.TrimSpace(it.Description)),
		Published:   parsePubDate(it.PubDate),
		URL:         u,
	}
}

func parsePubDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	t, err := time.Parse(time.RFC1123Z, s)
	if err != nil {
		// try the non-numeric-zone variant
		t, err = time.Parse(time.RFC1123, s)
		if err != nil {
			return ""
		}
	}
	return t.UTC().Format("2006-01-02 15:04")
}
