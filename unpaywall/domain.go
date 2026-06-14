package unpaywall

import (
	"context"
	"net/url"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes unpaywall as a kit Domain driver.
//
// A multi-domain host (ant) enables it with a single blank import:
//
//	import _ "github.com/tamnd/unpaywall-cli/unpaywall"
//
// The same Domain also builds the standalone unpaywall binary (see cli.NewApp).
func init() { kit.Register(Domain{}) }

// Domain is the unpaywall driver.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "unpaywall",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "unpaywall",
			Short:  "Check open access availability of academic papers via Unpaywall",
			Long: `unpaywall checks whether academic papers are freely available online
using the Unpaywall API (api.unpaywall.org). No API key or registration required.

OA status: gold (DOAJ journal), hybrid (open in subscription journal),
bronze (free on publisher site, no license), green (repository copy), closed.`,
			Site: "unpaywall.org",
			Repo: "https://github.com/tamnd/unpaywall-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{
		Name:    "doi",
		Group:   "lookup",
		Single:  true,
		Summary: "Look up open access status for a paper by DOI",
		Args:    []kit.Arg{{Name: "doi", Help: "DOI (e.g. 10.1038/nature12373)"}},
	}, lookupDOI)
}

// newClient builds the client from host-resolved config.
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

// --- inputs ---

type doiInput struct {
	DOI    string  `kit:"arg" help:"DOI to look up (e.g. 10.1038/nature12373)"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func lookupDOI(ctx context.Context, in doiInput, emit func(*Paper) error) error {
	p, err := in.Client.LookupDOI(ctx, in.DOI)
	if err != nil {
		return err
	}
	return emit(p)
}

// --- Resolver ---

// Classify turns an input into the canonical (type, id).
// A string starting with "10." is treated as a DOI; everything else is a query.
func (Domain) Classify(input string) (uriType, id string, err error) {
	if input == "" {
		return "", "", errs.Usage("empty unpaywall reference")
	}
	if strings.HasPrefix(input, "10.") {
		return "doi", input, nil
	}
	// full DOI URL: https://doi.org/10.xxx/yyy
	if u, e := url.Parse(input); e == nil && u.Host == "doi.org" {
		doi := strings.TrimPrefix(u.Path, "/")
		if strings.HasPrefix(doi, "10.") {
			return "doi", doi, nil
		}
	}
	return "query", input, nil
}

// Locate returns the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "doi":
		return "https://doi.org/" + id, nil
	case "query":
		return "https://unpaywall.org/search?query=" + url.QueryEscape(id), nil
	default:
		return "", errs.Usage("unpaywall has no resource type %q", uriType)
	}
}
