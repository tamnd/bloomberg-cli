package bloomberg

import (
	"encoding/xml"
	"html"
	"strings"
	"time"
)

// Article is the record emitted for each Bloomberg news item.
type Article struct {
	Rank         int    `json:"rank"          table:"RANK"`
	Title        string `json:"title"         table:"TITLE"`
	Description  string `json:"description"   table:"DESCRIPTION"`
	Published    string `json:"published"     table:"DATE"`
	Section      string `json:"section"       table:"SECTION"`
	ThumbnailURL string `json:"thumbnail_url" table:"-"`
	URL          string `json:"url"           table:"URL"`
}

// Section is the record emitted by the sections command.
type Section struct {
	Rank        int    `json:"rank"        table:"RANK"`
	Slug        string `json:"slug"        table:"SLUG"`
	Name        string `json:"name"        table:"NAME"`
	Description string `json:"description" table:"DESCRIPTION"`
	URL         string `json:"url"         table:"URL"`
}

// ─── wire types ─────────────────────────────────────────────────────────────

// rssDoc is the top-level RSS document envelope.
type rssDoc struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

// rssChannel holds channel-level metadata and all items.
type rssChannel struct {
	Title         string    `xml:"title"`
	Description   string    `xml:"description"`
	LastBuildDate string    `xml:"lastBuildDate"`
	Items         []rssItem `xml:"item"`
}

// rssItem is one <item> in the feed.
// Bloomberg feeds use:
//   - <![CDATA[...]]> for all text (transparent to encoding/xml)
//   - <dc:creator> for author names
//   - <media:content><media:thumbnail url="..."> for thumbnail images
//   - <category domain="stock-symbol"> for ticker symbols
//
// Namespace prefixes in struct tags use the local name only; encoding/xml
// matches by local name when the full namespace URI is not specified.
type rssItem struct {
	Title        string          `xml:"title"`
	Link         string          `xml:"link"`
	GUID         string          `xml:"guid"`
	PubDate      string          `xml:"pubDate"`
	Description  string          `xml:"description"`
	Creator      string          `xml:"creator"`       // dc:creator
	Categories   []rssCategory   `xml:"category"`
	MediaContent rssMediaContent `xml:"content"`       // media:content
}

// rssCategory is a <category> tag. Bloomberg uses domain="stock-symbol".
type rssCategory struct {
	Domain string `xml:"domain,attr"`
	Value  string `xml:",chardata"`
}

// rssMediaContent is <media:content> which wraps a <media:thumbnail>.
type rssMediaContent struct {
	URL       string       `xml:"url,attr"`
	Thumbnail rssThumbnail `xml:"thumbnail"` // media:thumbnail
}

// rssThumbnail is <media:thumbnail url="...">.
type rssThumbnail struct {
	URL string `xml:"url,attr"`
}

// ─── conversion helpers ──────────────────────────────────────────────────────

func itemToArticle(it rssItem, rank int, section string) Article {
	u := strings.TrimSpace(it.Link)
	if u == "" {
		u = strings.TrimSpace(it.GUID)
	}
	thumb := it.MediaContent.Thumbnail.URL
	if thumb == "" {
		thumb = it.MediaContent.URL
	}
	return Article{
		Rank:         rank,
		Title:        html.UnescapeString(strings.TrimSpace(it.Title)),
		Description:  html.UnescapeString(strings.TrimSpace(it.Description)),
		Published:    parsePubDate(it.PubDate),
		Section:      section,
		ThumbnailURL: thumb,
		URL:          u,
	}
}

// parsePubDate parses an RSS pubDate string and formats it as "2006-01-02 15:04" UTC.
// Returns "" on any parse error or empty input.
//
// Bloomberg live feeds use RFC1123 with GMT suffix:
//
//	Mon, 15 Jun 2026 02:18:04 GMT
//
// The parser tries RFC1123Z first (numeric offset), then RFC1123 (named zone).
func parsePubDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	t, err := time.Parse(time.RFC1123Z, s)
	if err != nil {
		t, err = time.Parse(time.RFC1123, s)
		if err != nil {
			return ""
		}
	}
	return t.UTC().Format("2006-01-02 15:04")
}
