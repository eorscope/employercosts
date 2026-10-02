// Package employercosts embeds the EOR Scope dataset of statutory employer
// costs (76 countries) and published Employer of Record provider plans, and
// exposes it as typed, read-only records. It has no dependencies and makes no
// network calls.
//
// The package is a lookup library: it returns the figures as exported and
// computes nothing. Employer cost totals are the ones published in the
// dataset; the package does not recompute them. For a cost at another
// salary, use the calculator at https://eorscope.com/eor-cost-calculator/.
//
//   - Countries, Country and CountriesInRegion return one record per country.
//   - Contributions returns the employer contributions and statutory extras
//     of a country.
//   - Assumptions returns the case each country's lines and total hold for.
//   - Providers and Provider return provider plans.
//
// A country flagged TotalDeclaredFloor publishes an "at least" figure, not a
// total. Under TotalDeclaredCeiling the modelled contribution base is the
// highest the law allows, and lines with InTotal false come on top. A
// contribution's Rate applies to its Base times BaseFactor (1 unless the law
// builds the base from more than the salary, as Mexico's integrated base
// does), before BaseFloorAnnualLocal and BaseCapAnnualLocal.
//
// Sources and method are described at https://eorscope.com/methodology/ and
// each country has a page under https://eorscope.com/employer-of-record/ with
// its sources. The figures are a cost comparison, not legal or tax advice.
//
// Data licence: the embedded dataset is licensed under CC BY 4.0
// (https://creativecommons.org/licenses/by/4.0/). Attribution: EOR Scope,
// published by Trésor Kaya EI (Les Créavores), France. The Go code is MIT
// licensed.
package employercosts
