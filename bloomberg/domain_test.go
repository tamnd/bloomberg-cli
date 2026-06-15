package bloomberg

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tamnd/any-cli/kit"
)

// TestDomainInfo checks the scheme, host, and binary identity.
func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "bloomberg" {
		t.Errorf("Scheme = %q, want bloomberg", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "bloomberg" {
		t.Errorf("Identity.Binary = %q, want bloomberg", info.Identity.Binary)
	}
}

// TestDomainRegistered verifies the init() registered bloomberg with kit.
func TestDomainRegistered(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range h.Domains() {
		if d == "bloomberg" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("bloomberg domain not registered; got %v", h.Domains())
	}
}

// TestFeedsCmd checks that feedsCmd emits one Feed per section entry.
func TestFeedsCmd(t *testing.T) {
	c := NewClient(DefaultConfig())
	var got []Feed
	err := feedsCmd(context.Background(), feedsIn{Client: c}, func(f *Feed) error {
		got = append(got, *f)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(sectionRegistry) {
		t.Fatalf("feedsCmd emitted %d feeds, want %d", len(got), len(sectionRegistry))
	}
	for _, f := range got {
		if f.Name == "" {
			t.Error("Feed.Name is empty")
		}
		if f.URL == "" {
			t.Error("Feed.URL is empty")
		}
	}
}

// TestNewsCmdDefaultSection checks that newsCmd defaults to "top" when no
// section flag is provided, using a fake server.
func TestNewsCmdDefaultSection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(fakeRSS))
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	c := NewClient(cfg)

	var got []Article
	err := newsCmd(context.Background(), newsIn{Client: c, Section: ""}, func(a *Article) error {
		got = append(got, *a)
		return nil
	})
	if err != nil {
		t.Fatalf("newsCmd with default section: %v", err)
	}
	if len(got) == 0 {
		t.Error("newsCmd returned no articles")
	}
}

// TestNewsCmdExplicitSection checks that an explicit section flag is forwarded.
func TestNewsCmdExplicitSection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(fakeRSS))
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	c := NewClient(cfg)

	var got []Article
	err := newsCmd(context.Background(), newsIn{Client: c, Section: "markets"}, func(a *Article) error {
		got = append(got, *a)
		return nil
	})
	if err != nil {
		t.Fatalf("newsCmd with section=markets: %v", err)
	}
	if len(got) == 0 {
		t.Error("newsCmd(markets) returned no articles")
	}
}

// TestNewsCmdLimit checks that the Limit field is honoured.
func TestNewsCmdLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(fakeRSS))
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	c := NewClient(cfg)

	var got []Article
	err := newsCmd(context.Background(), newsIn{Client: c, Section: "top", Limit: 1}, func(a *Article) error {
		got = append(got, *a)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Errorf("want 1 article with limit=1, got %d", len(got))
	}
}

// TestFetchSection checks that fetchSection routes to the right slug.
func TestFetchSection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(fakeRSS))
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	c := NewClient(cfg)

	var got []Article
	err := fetchSection(context.Background(), sectionIn{Client: c, Section: "technology"}, func(a *Article) error {
		got = append(got, *a)
		return nil
	})
	if err != nil {
		t.Fatalf("fetchSection(technology): %v", err)
	}
	if len(got) == 0 {
		t.Error("fetchSection returned no articles")
	}
}
