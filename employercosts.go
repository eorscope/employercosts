package employercosts

import (
	"bytes"
	"embed"
	"encoding/csv"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed data/*.csv
var files embed.FS

// CountrySummary is one row of country_summary.csv. Optional numbers are nil when
// the dataset leaves them empty; dates are zero when empty.
type CountrySummary struct {
	ISO                    string   // ISO 3166-1 alpha-2
	Name                   string   // country name
	Region                 string   // region, see Regions
	Currency               string   // ISO 4217 local currency
	ExampleSalaryUSD       int      // illustrative gross annual salary in USD (an example, not a median)
	ExampleRole            string   // illustrative role
	ExamplePlace           string   // state, province or city the lines are priced for, when not national
	BasicShareOfGross      *float64 // share of gross taken as basic pay by lines with Base "basic"
	EmployerCostPct        float64  // statutory employer cost, percent of gross at the example salary
	EmployerCostPctPrinted float64  // the same percent as printed on eorscope.com (1 decimal)
	// TotalDeclaredFloor: the employer cost is an "at least" figure, not a total.
	TotalDeclaredFloor bool
	// TotalDeclaredCeiling: the modelled contribution base is the highest the
	// law allows; lines with InTotal false come on top.
	TotalDeclaredCeiling bool
	EmployerCostUSDMonth float64   // statutory employer cost per month in USD at the example salary
	TotalExcludes        string    // a cost the total leaves out
	FXLocalPerUSD        float64   // local currency units per 1 USD used for the conversion
	FXDate               time.Time // date of the exchange rate (zero for USD)
	LastReviewed         time.Time // date the country data was last verified
}

// Contribution is one row of country_contributions.csv: an employer
// contribution (LineType "contribution") or a statutory extra
// (LineType "statutory_extra").
type Contribution struct {
	ISO                  string
	Country              string
	Currency             string
	LineType             string // contribution | statutory_extra
	LineID               string // unique within the country and line type
	LineName             string
	Rate                 *float64 // decimal (0.12 = 12%); nil for statutory extras
	Base                 string   // gross | basic | flat | band | "" (statutory extras)
	BaseFactor           *float64 // multiplier on the base, applied before floor and cap (1 = the salary itself); nil for flat lines and statutory extras
	BaseFloorAnnualLocal *float64 // yearly floor on the base, local currency
	BaseCapAnnualLocal   *float64 // yearly ceiling on the base, local currency
	FlatAnnualLocal      *float64 // fixed yearly amount when Base is "flat"
	MinGrossAnnualLocal  *float64 // line applies only when annual gross is at least this
	MaxGrossAnnualLocal  *float64 // line applies only when annual gross is at most this
	ExtraKind            string   // percent | months | days | "" (contributions)
	ExtraValue           *float64 // value in the unit given by ExtraKind
	SecondarySource      bool     // the line rests, in whole or in part, on a non-official source
	ThirteenthMonth      bool     // the country mandates a 13th-month salary
	TotalDeclaredFloor   bool     // the country's employer cost is an "at least" figure
	TotalDeclaredCeiling bool     // the modelled base is the highest the law allows
	InTotal              bool     // the line is included in the employer cost total
	Applies              string   // condition in plain words
	Notes                string
	SourceName           string
	SourceURL            string
	SourceCheckedAt      time.Time
}

// Assumption is one row of country_assumptions.csv: the case a country's
// lines and total are priced for.
type Assumption struct {
	ISO   string
	ID    string // unique within the country
	Label string
	Value float64 // a share, a rate, a threshold, or 1 for a stated condition
	Notes string
}

type dataset struct {
	countries     []CountrySummary
	contributions map[string][]Contribution
	assumptions   map[string][]Assumption
}

var (
	loadOnce sync.Once
	ds       *dataset
)

func data() *dataset {
	loadOnce.Do(func() {
		d, err := load()
		if err != nil {
			panic("employercosts: embedded dataset: " + err.Error())
		}
		ds = d
	})
	return ds
}

// Countries returns every country, sorted by ISO code.
func Countries() []CountrySummary {
	return append([]CountrySummary(nil), data().countries...)
}

// Country returns the country with the given ISO 3166-1 alpha-2 code
// (case-insensitive).
func Country(iso string) (CountrySummary, bool) {
	iso = strings.ToUpper(iso)
	for _, c := range data().countries {
		if c.ISO == iso {
			return c, true
		}
	}
	return CountrySummary{}, false
}

// Regions returns the distinct regions, sorted.
func Regions() []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range data().countries {
		if !seen[c.Region] {
			seen[c.Region] = true
			out = append(out, c.Region)
		}
	}
	sort.Strings(out)
	return out
}

// CountriesInRegion returns the countries of a region (case-insensitive),
// sorted by ISO code.
func CountriesInRegion(region string) []CountrySummary {
	var out []CountrySummary
	for _, c := range data().countries {
		if strings.EqualFold(c.Region, region) {
			out = append(out, c)
		}
	}
	return out
}

// Contributions returns the contribution and statutory extra lines of a
// country, in dataset order.
func Contributions(iso string) []Contribution {
	return append([]Contribution(nil), data().contributions[strings.ToUpper(iso)]...)
}

// Assumptions returns the declared assumptions of a country, in dataset order.
func Assumptions(iso string) []Assumption {
	return append([]Assumption(nil), data().assumptions[strings.ToUpper(iso)]...)
}

// --- parsing ---

type row struct {
	idx  map[string]int
	rec  []string
	err  error
	line int
}

func (r *row) str(col string) string {
	i, ok := r.idx[col]
	if !ok {
		r.fail(fmt.Errorf("missing column %q", col))
		return ""
	}
	return r.rec[i]
}

func (r *row) fail(err error) {
	if r.err == nil {
		r.err = fmt.Errorf("line %d: %w", r.line, err)
	}
}

func (r *row) optFloat(col string) *float64 {
	s := r.str(col)
	if s == "" {
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		r.fail(fmt.Errorf("%s: %w", col, err))
		return nil
	}
	return &f
}

func (r *row) float(col string) float64 {
	if f := r.optFloat(col); f != nil {
		return *f
	}
	r.fail(fmt.Errorf("%s: empty", col))
	return 0
}

func (r *row) optInt(col string) *int {
	s := r.str(col)
	if s == "" {
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		r.fail(fmt.Errorf("%s: %w", col, err))
		return nil
	}
	return &n
}

func (r *row) boolean(col string) bool {
	b, err := strconv.ParseBool(r.str(col))
	if err != nil {
		r.fail(fmt.Errorf("%s: %w", col, err))
	}
	return b
}

func (r *row) date(col string) time.Time {
	s := r.str(col)
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		r.fail(fmt.Errorf("%s: %w", col, err))
	}
	return t
}

func (r *row) list(col, sep string) []string {
	s := r.str(col)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, sep)
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func readCSV(name string, fn func(*row)) error {
	b, err := files.ReadFile("data/" + name)
	if err != nil {
		return err
	}
	recs, err := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF}))).ReadAll()
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if len(recs) == 0 {
		return fmt.Errorf("%s: empty", name)
	}
	idx := map[string]int{}
	for i, h := range recs[0] {
		idx[h] = i
	}
	for i, rec := range recs[1:] {
		r := &row{idx: idx, rec: rec, line: i + 2}
		fn(r)
		if r.err != nil {
			return fmt.Errorf("%s: %w", name, r.err)
		}
	}
	return nil
}

func load() (*dataset, error) {
	d := &dataset{contributions: map[string][]Contribution{}, assumptions: map[string][]Assumption{}}
	err := readCSV("country_summary.csv", func(r *row) {
		c := CountrySummary{
			ISO:                    r.str("iso"),
			Name:                   r.str("country"),
			Region:                 r.str("region"),
			Currency:               r.str("currency"),
			ExampleRole:            r.str("example_role"),
			ExamplePlace:           r.str("example_place"),
			BasicShareOfGross:      r.optFloat("basic_share_of_gross"),
			EmployerCostPct:        r.float("employer_cost_pct"),
			EmployerCostPctPrinted: r.float("employer_cost_pct_printed"),
			TotalDeclaredFloor:     r.boolean("total_declared_floor"),
			TotalDeclaredCeiling:   r.boolean("total_declared_ceiling"),
			EmployerCostUSDMonth:   r.float("employer_cost_usd_month"),
			TotalExcludes:          r.str("total_excludes"),
			FXLocalPerUSD:          r.float("fx_local_per_usd"),
			FXDate:                 r.date("fx_date"),
			LastReviewed:           r.date("last_reviewed"),
		}
		if n := r.optInt("example_salary_usd"); n != nil {
			c.ExampleSalaryUSD = *n
		} else {
			r.fail(fmt.Errorf("example_salary_usd: empty"))
		}
		d.countries = append(d.countries, c)
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(d.countries, func(i, j int) bool { return d.countries[i].ISO < d.countries[j].ISO })

	err = readCSV("country_contributions.csv", func(r *row) {
		c := Contribution{
			ISO:                  r.str("iso"),
			Country:              r.str("country"),
			Currency:             r.str("currency"),
			LineType:             r.str("line_type"),
			LineID:               r.str("line_id"),
			LineName:             r.str("line_name"),
			Rate:                 r.optFloat("rate"),
			Base:                 r.str("base"),
			BaseFactor:           r.optFloat("base_factor"),
			BaseFloorAnnualLocal: r.optFloat("base_floor_annual_local"),
			BaseCapAnnualLocal:   r.optFloat("base_cap_annual_local"),
			FlatAnnualLocal:      r.optFloat("flat_annual_local"),
			MinGrossAnnualLocal:  r.optFloat("min_gross_annual_local"),
			MaxGrossAnnualLocal:  r.optFloat("max_gross_annual_local"),
			ExtraKind:            r.str("extra_kind"),
			ExtraValue:           r.optFloat("extra_value"),
			SecondarySource:      r.boolean("secondary_source"),
			ThirteenthMonth:      r.boolean("thirteenth_month"),
			TotalDeclaredFloor:   r.boolean("total_declared_floor"),
			TotalDeclaredCeiling: r.boolean("total_declared_ceiling"),
			InTotal:              r.boolean("in_total"),
			Applies:              r.str("applies"),
			Notes:                r.str("notes"),
			SourceName:           r.str("source_name"),
			SourceURL:            r.str("source_url"),
			SourceCheckedAt:      r.date("source_checked_at"),
		}
		d.contributions[c.ISO] = append(d.contributions[c.ISO], c)
	})
	if err != nil {
		return nil, err
	}

	err = readCSV("country_assumptions.csv", func(r *row) {
		a := Assumption{
			ISO:   r.str("iso"),
			ID:    r.str("id"),
			Label: r.str("label"),
			Value: r.float("value"),
			Notes: r.str("notes"),
		}
		d.assumptions[a.ISO] = append(d.assumptions[a.ISO], a)
	})
	if err != nil {
		return nil, err
	}

	return d, nil
}
