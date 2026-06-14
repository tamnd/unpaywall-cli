package unpaywall_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tamnd/unpaywall-cli/unpaywall"
)

const fakeDOIJSON = `{
  "doi": "10.1038/nature12373",
  "title": "Nanometre-scale thermometry in a living cell",
  "journal_name": "Nature",
  "published_date": "2013-07-30",
  "year": 2013,
  "is_oa": true,
  "oa_status": "bronze",
  "best_oa_location": {
    "url": "https://www.nature.com/articles/nature12373",
    "url_for_pdf": "https://www.nature.com/articles/nature12373.pdf",
    "host_type": "publisher",
    "license": null,
    "version": "publishedVersion"
  },
  "oa_locations": []
}`

func newTestClient(ts *httptest.Server) *unpaywall.Client {
	cfg := unpaywall.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	return unpaywall.NewClient(cfg)
}

func TestLookupDOI(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fakeDOIJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	p, err := c.LookupDOI(context.Background(), "10.1038/nature12373")
	if err != nil {
		t.Fatal(err)
	}
	if p.DOI != "10.1038/nature12373" {
		t.Errorf("DOI = %q, want 10.1038/nature12373", p.DOI)
	}
	if p.Title != "Nanometre-scale thermometry in a living cell" {
		t.Errorf("Title = %q", p.Title)
	}
	if p.Journal != "Nature" {
		t.Errorf("Journal = %q, want Nature", p.Journal)
	}
	if p.Published != "2013-07-30" {
		t.Errorf("Published = %q, want 2013-07-30", p.Published)
	}
	if p.Year != 2013 {
		t.Errorf("Year = %d, want 2013", p.Year)
	}
	if !p.IsOA {
		t.Error("IsOA = false, want true")
	}
	if p.OAStatus != "bronze" {
		t.Errorf("OAStatus = %q, want bronze", p.OAStatus)
	}
	if p.BestURL != "https://www.nature.com/articles/nature12373" {
		t.Errorf("BestURL = %q", p.BestURL)
	}
	if p.BestPDFURL != "https://www.nature.com/articles/nature12373.pdf" {
		t.Errorf("BestPDFURL = %q", p.BestPDFURL)
	}
	if p.HostType != "publisher" {
		t.Errorf("HostType = %q, want publisher", p.HostType)
	}
	if p.License != "" {
		t.Errorf("License = %q, want empty (null license)", p.License)
	}
}

func TestLookupDOI404(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"error":"doi not found"}`)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.LookupDOI(context.Background(), "10.9999/does-not-exist")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err.Error())
	}
}

func TestLookupDOIEmailParam(t *testing.T) {
	var gotQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = fmt.Fprint(w, fakeDOIJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.LookupDOI(context.Background(), "10.1038/nature12373")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotQuery, "email=") {
		t.Errorf("query %q does not contain email param", gotQuery)
	}
	if !strings.Contains(gotQuery, "tamnd87") {
		t.Errorf("query %q does not contain expected email", gotQuery)
	}
}

func TestRetry503(t *testing.T) {
	var hits int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = fmt.Fprint(w, fakeDOIJSON)
	}))
	defer ts.Close()

	cfg := unpaywall.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	cfg.Retries = 3
	c := unpaywall.NewClient(cfg)

	_, err := c.LookupDOI(context.Background(), "10.1038/nature12373")
	if err != nil {
		t.Fatal(err)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3 (2 failures then success)", hits)
	}
}
