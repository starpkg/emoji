#!/usr/bin/env bash
# Re-download the pinned emoji data sources into data/sources/.
#
# This is the only step that touches the network. After running it, regenerate
# the Go table with `go generate ./...` and review the diff. To bump a source,
# change its version below AND the matching provenance constant in
# internal/gen/main.go, then re-run this script + `go generate ./...`.
set -euo pipefail

# Pinned versions (keep in sync with internal/gen/main.go and data/SOURCES.md).
CARPEDM20_VERSION="v2.15.0"
GEMOJI_VERSION="v4.1.0"

cd "$(dirname "$0")/sources"

fetch() {
	local url="$1" out="$2"
	echo "fetching $out  <-  $url"
	curl -fsSL "$url" -o "$out"
}

fetch "https://raw.githubusercontent.com/carpedm20/emoji/${CARPEDM20_VERSION}/emoji/unicode_codes/emoji.json" \
	"carpedm20-emoji-${CARPEDM20_VERSION}.json"
fetch "https://raw.githubusercontent.com/carpedm20/emoji/${CARPEDM20_VERSION}/LICENSE.txt" \
	"carpedm20-emoji-LICENSE.txt"

fetch "https://raw.githubusercontent.com/github/gemoji/${GEMOJI_VERSION}/db/emoji.json" \
	"gemoji-${GEMOJI_VERSION}.json"
fetch "https://raw.githubusercontent.com/github/gemoji/${GEMOJI_VERSION}/LICENSE" \
	"gemoji-LICENSE.txt"

echo "done. next: go generate ./... && go test ./..."
