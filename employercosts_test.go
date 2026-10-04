package employercosts

import (
	"regexp"
	"testing"
)

var isoRe = regexp.MustCompile(`^[A-Z]{2}$`)
var curRe = regexp.MustCompile(`^[A-Z]{3}$`)

func in(s string, set ...string) bool {
	for _, v := range set {
		if s == v {
			return true
		}
	}
	return false
}

func TestCountries(t *testing.T) {
	cs := Countries()
	if len(cs) != 76 {
		t.Fatalf("Countries: got %d, want 76", len(cs))
	}
	seen := map[string]bool{}
	for _, c := range cs {
		if !isoRe.MatchString(c.ISO) || seen[c.ISO] {
			t.Errorf("bad or duplicate ISO %q", c.ISO)
		}
		seen[c.ISO] = true
		if c.Name == "" || c.Region == "" || !curRe.MatchString(c.Currency) {
			t.Errorf("%s: empty name/region or bad currency %q", c.ISO, c.Currency)
		}
		if c.ExampleSalaryUSD <= 0 || c.FXLocalPerUSD <= 0 || c.EmployerCostPct < 0 || c.EmployerCostUSDMonth < 0 {
			t.Errorf("%s: non-positive number", c.ISO)
		}
		if c.LastReviewed.IsZero() {
			t.Errorf("%s: no last_reviewed", c.ISO)
		}
		if c.Currency != "USD" && c.FXDate.IsZero() {
			t.Errorf("%s: no fx_date for %s", c.ISO, c.Currency)
		}
		if s := c.BasicShareOfGross; s != nil && (*s <= 0 || *s > 1) {
			t.Errorf("%s: basic_share_of_gross %v", c.ISO, *s)
		}
		got, ok := Country(c.ISO)
		if !ok || got.ISO != c.ISO {
			t.Errorf("Country(%q) not found", c.ISO)
		}
	}
	if _, ok := Country("xx"); ok {
		t.Error(`Country("xx") found`)
	}
	if c, ok := Country("de"); !ok || c.ISO != "DE" {
		t.Error(`Country("de") is not case-insensitive`)
	}
}

func TestRegions(t *testing.T) {
	n := 0
	for _, r := range Regions() {
		cs := CountriesInRegion(r)
		if len(cs) == 0 {
			t.Errorf("region %q has no country", r)
		}
		for _, c := range cs {
			if c.Region != r {
				t.Errorf("%s in %q, want %q", c.ISO, c.Region, r)
			}
		}
		n += len(cs)
	}
	if n != len(Countries()) {
		t.Errorf("regions cover %d countries, want %d", n, len(Countries()))
	}
}

func TestContributions(t *testing.T) {
	total := 0
	factored := map[float64]int{}
	for _, c := range Countries() {
		lines := Contributions(c.ISO)
		if len(lines) == 0 {
			t.Errorf("%s: no contribution line", c.ISO)
		}
		total += len(lines)
		for _, l := range lines {
			if l.ISO != c.ISO || l.Currency != c.Currency || l.Country != c.Name {
				t.Errorf("%s/%s: iso, country or currency differs from summary", c.ISO, l.LineID)
			}
			if l.TotalDeclaredFloor != c.TotalDeclaredFloor || l.TotalDeclaredCeiling != c.TotalDeclaredCeiling {
				t.Errorf("%s/%s: floor/ceiling flags differ from summary", c.ISO, l.LineID)
			}
			switch l.LineType {
			case "contribution":
				if !in(l.Base, "gross", "basic", "flat", "band") || l.ExtraKind != "" {
					t.Errorf("%s/%s: contribution base %q extra %q", c.ISO, l.LineID, l.Base, l.ExtraKind)
				}
				if (l.Base == "flat") != (l.BaseFactor == nil) {
					t.Errorf("%s/%s: base %q with base_factor %v", c.ISO, l.LineID, l.Base, l.BaseFactor)
				}
				if l.BaseFactor != nil && *l.BaseFactor != 1 {
					factored[*l.BaseFactor]++
				}
			case "statutory_extra":
				if !in(l.ExtraKind, "percent", "months", "days") || l.ExtraValue == nil || l.Rate != nil || l.BaseFactor != nil {
					t.Errorf("%s/%s: statutory extra kind %q", c.ISO, l.LineID, l.ExtraKind)
				}
			default:
				t.Errorf("%s/%s: line_type %q", c.ISO, l.LineID, l.LineType)
			}
			if l.LineID == "" || l.SourceURL == "" || l.SourceCheckedAt.IsZero() {
				t.Errorf("%s/%s: missing id, source or date", c.ISO, l.LineID)
			}
		}
	}
	if total != 542 {
		t.Errorf("contribution lines: got %d, want 542", total)
	}
	// Mexico: 9 lines on the integrated base (SBC), 1 on the state payroll-tax base
	if len(factored) != 2 || factored[1.049315] != 9 || factored[1.049867] != 1 {
		t.Errorf("lines with a base_factor other than 1: %v", factored)
	}
	if Contributions("xx") != nil {
		t.Error(`Contributions("xx") not empty`)
	}
}

func TestAssumptions(t *testing.T) {
	total := 0
	for _, c := range Countries() {
		for _, a := range Assumptions(c.ISO) {
			if a.ISO != c.ISO || a.ID == "" || a.Label == "" {
				t.Errorf("%s: bad assumption %+v", c.ISO, a)
			}
			total++
		}
	}
	if total != 108 {
		t.Errorf("assumptions on known countries: got %d, want 108", total)
	}
}
