// Package unpaywall is the library behind the unpaywall command line:
// the HTTP client, request shaping, and the typed data models for the
// Unpaywall public open-access API (api.unpaywall.org/v2).
//
// The API requires no registration — pass your email as the identifying key.
// It is polite to pace requests to at most one per second.
package unpaywall

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/tamnd/any-cli/kit/errs"
)

// Host is the API hostname.
const Host = "api.unpaywall.org"

// BaseURL is the v2 API root every request is built from.
const BaseURL = "https://api.unpaywall.org/v2"

// Config holds all tunable parameters for the Client.
type Config struct {
	BaseURL   string
	Email     string
	UserAgent string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
}

// DefaultConfig returns a Config with sensible, polite defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   BaseURL,
		Email:     "tamnd87@gmail.com",
		UserAgent: "tamnd-unpaywall-cli/0.1 (tamnd87@gmail.com)",
		Rate:      time.Second,
		Retries:   3,
		Timeout:   15 * time.Second,
	}
}

// Client talks to the Unpaywall API over HTTP.
type Client struct {
	cfg  Config
	http *http.Client
	mu   sync.Mutex
	last time.Time
}

// NewClient returns a Client configured with cfg.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// LookupDOI fetches the open access status for a paper by its DOI.
// A 404 from the API is returned as errs.NotFound.
func (c *Client) LookupDOI(ctx context.Context, doi string) (*Paper, error) {
	u := c.cfg.BaseURL + "/" + url.PathEscape(doi) + "?email=" + url.QueryEscape(c.cfg.Email)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var w wirePaper
	if err := json.Unmarshal(body, &w); err != nil {
		return nil, fmt.Errorf("decode doi %q: %w", doi, err)
	}
	return paperFromWire(w), nil
}

// --- output types ---

// Paper is the canonical output record for a DOI lookup.
type Paper struct {
	DOI        string `json:"doi" kit:"id"`
	Title      string `json:"title"`
	Journal    string `json:"journal"`
	Published  string `json:"published"`
	Year       int    `json:"year"`
	IsOA       bool   `json:"is_oa"`
	OAStatus   string `json:"oa_status"`
	BestURL    string `json:"best_url"`
	BestPDFURL string `json:"best_pdf_url"`
	HostType   string `json:"host_type"`
	License    string `json:"license"`
}

// --- wire types ---

type wirePaper struct {
	DOI            string          `json:"doi"`
	Title          string          `json:"title"`
	JournalName    string          `json:"journal_name"`
	PublishedDate  string          `json:"published_date"`
	Year           int             `json:"year"`
	IsOA           bool            `json:"is_oa"`
	OAStatus       string          `json:"oa_status"`
	BestOALocation *wireOALocation `json:"best_oa_location"`
	OALocations    []wireOALocation `json:"oa_locations"`
}

type wireOALocation struct {
	URL       string  `json:"url"`
	URLForPDF string  `json:"url_for_pdf"`
	HostType  string  `json:"host_type"`
	License   *string `json:"license"`
	Version   string  `json:"version"`
}

func paperFromWire(w wirePaper) *Paper {
	p := &Paper{
		DOI:       w.DOI,
		Title:     w.Title,
		Journal:   w.JournalName,
		Published: w.PublishedDate,
		Year:      w.Year,
		IsOA:      w.IsOA,
		OAStatus:  w.OAStatus,
	}
	if loc := w.BestOALocation; loc != nil {
		p.BestURL = loc.URL
		p.BestPDFURL = loc.URLForPDF
		p.HostType = loc.HostType
		if loc.License != nil {
			p.License = *loc.License
		}
	}
	return p
}

// --- HTTP transport ---

func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
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
	req.Header.Set("User-Agent", c.cfg.UserAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, false, errs.NotFound("doi not found (HTTP 404)")
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	return min(time.Duration(attempt)*500*time.Millisecond, 5*time.Second)
}
