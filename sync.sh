#!/bin/sh
# Copies the CC BY 4.0 dataset export into data/ (embedded by the package).
# Run after each dataset regeneration, then: go vet ./... && go test ./...
set -eu
here=$(cd "$(dirname "$0")" && pwd)
src=${1:-"$here/../../dataset/eorscope-employer-costs"}
for f in country_summary.csv country_contributions.csv country_assumptions.csv LICENSE; do
	cp "$src/$f" "$here/data/$f"
done
echo "synced from $src"
