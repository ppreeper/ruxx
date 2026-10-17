#!/usr/bin/env bash
#
# bench.sh runs the ruxx benchmark suite, and — when a corpus is available —
# times a full-tree lint sweep. See docs/PERFORMANCE.md for the numbers this
# produced.
#
# Usage:
#   scripts/bench.sh
#   BENCHTIME=1s scripts/bench.sh
#   RXX_CORPUS=/path/to/dirs.txt scripts/bench.sh
#
# RXX_CORPUS points at a file with one directory root per line. Each root is
# walked recursively; the sweep uses `-n` so it measures real work rather than
# cache lookups.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

benchtime="${BENCHTIME:-300ms}"
echo "== benchmarks (-benchtime=$benchtime) =="
go test -run '^$' -bench . -benchtime="$benchtime" ./internal/...

corpus="${RXX_CORPUS:-}"
if [[ -z "$corpus" && -f /tmp/opencode/dirs4.txt ]]; then
	corpus=/tmp/opencode/dirs4.txt
fi
if [[ -z "$corpus" || ! -f "$corpus" ]]; then
	echo
	echo "== corpus sweep skipped: set RXX_CORPUS=/path/to/dirs.txt =="
	exit 0
fi

bin="$(mktemp -d)/ruxx"
go build -o "$bin" ./cmd/ruxx
mapfile -t dirs <"$corpus"

echo
echo "== corpus sweep: ${#dirs[@]} roots =="
files="$(find "${dirs[@]}" -name '*.xml' | wc -l)"
echo "xml files: $files"

out="$(mktemp)"
echo "--- run 1 (cold) ---"
time "$bin" check -n "${dirs[@]}" >"$out"
echo "findings: $(wc -l <"$out")"

echo "--- run 2 (cold, repeat) ---"
time "$bin" check -n "${dirs[@]}" >"$out"
echo "findings: $(wc -l <"$out")"
