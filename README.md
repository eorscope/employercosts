# employercosts

Dependency-free Go package that embeds the EOR Scope dataset of statutory
employer costs and Employer of Record provider plans, as typed, read-only
records.

- 76 countries: employer cost in % and USD per month at an example salary,
  local currency, exchange rate and review date
- 542 employer contribution and statutory extra lines, each with rate, base,
  floor/ceiling, source URL and date
- 108 declared assumptions (the case each country's total holds for)
- 35 provider plans with their published pricing model, billing and source

```go
import "github.com/eorscope/employercosts"

de, ok := employercosts.Country("DE")
lines := employercosts.Contributions("DE")
europe := employercosts.CountriesInRegion("Europe")
plans, ok := employercosts.Provider("deel")
```

The package is a lookup library: it returns the figures as exported and
computes nothing. Countries flagged `TotalDeclaredFloor` publish an "at least"
figure, not a total; under `TotalDeclaredCeiling` the modelled contribution
base is the highest the law allows and lines with `InTotal` false come on top.
For a cost at another salary, use the
[EOR cost calculator](https://eorscope.com/eor-cost-calculator/). Sources and
method: [methodology](https://eorscope.com/methodology/).

Cost comparison, not legal or tax advice.

## Updating the data

`sync.sh` copies the four CSV files and the data licence from
`../../dataset/eorscope-employer-costs/` (or the directory given as first
argument) into `data/`. Then run `go vet ./... && go test ./...`.

## Licence

Code: MIT, see LICENSE.

Data (`data/`): CC BY 4.0, see data/LICENSE. Attribution: EOR Scope,
published by Trésor Kaya EI (Les Créavores), France.
