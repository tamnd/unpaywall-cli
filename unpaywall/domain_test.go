package unpaywall

import (
	"testing"
)

// These tests are offline: they exercise the URI driver's pure string functions.
// HTTP behaviour is covered in unpaywall_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "unpaywall" {
		t.Errorf("Scheme = %q, want unpaywall", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "unpaywall" {
		t.Errorf("Identity.Binary = %q, want unpaywall", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		in  string
		typ string
		id  string
	}{
		{"10.1038/nature12373", "doi", "10.1038/nature12373"},
		{"10.7717/peerj.4375", "doi", "10.7717/peerj.4375"},
		{"https://doi.org/10.1038/nature12373", "doi", "10.1038/nature12373"},
		{"einstein", "query", "einstein"},
		{"open access", "query", "open access"},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if err != nil || typ != tc.typ || id != tc.id {
			t.Errorf("Classify(%q) = (%q, %q, %v), want (%q, %q, nil)",
				tc.in, typ, id, err, tc.typ, tc.id)
		}
	}
}

func TestClassifyEmpty(t *testing.T) {
	_, _, err := Domain{}.Classify("")
	if err == nil {
		t.Error("expected error for empty input, got nil")
	}
}

func TestLocateDOI(t *testing.T) {
	got, err := Domain{}.Locate("doi", "10.1038/nature12373")
	want := "https://doi.org/10.1038/nature12373"
	if err != nil || got != want {
		t.Errorf("Locate = (%q, %v), want (%q, nil)", got, err, want)
	}
}

func TestLocateQuery(t *testing.T) {
	got, err := Domain{}.Locate("query", "einstein")
	if err != nil {
		t.Fatalf("Locate error: %v", err)
	}
	if got == "" {
		t.Error("Locate returned empty URL for query")
	}
}

func TestLocateUnknownType(t *testing.T) {
	_, err := Domain{}.Locate("unknown", "foo")
	if err == nil {
		t.Error("expected error for unknown type, got nil")
	}
}
