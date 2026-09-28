#!/bin/sh
# Build the browser demo: gag compiled to WebAssembly, on a page where
# xterm.js is the terminal. The output is static files; serve it from
# anywhere, under any path.
#
#     web/build.sh [outdir]      (default: web/dist)
#
# Needs go and npm. Bubble Tea v1 has no js version of its terminal code, so
# the build uses a temporary copy of it with the stubs in web/_bubbletea/,
# through a temporary go.mod. The repo's go.mod is never touched, which keeps
# `go install ...@latest` working.
set -eu

XTERM=6.0.0 XTERM_FIT=0.11.0 XTERM_WEBGL=0.19.0

cd "$(dirname "$0")/.."
OUT=${1:-web/dist}
GO=${GO:-go}
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

# Bubble Tea, patched.
$GO mod download github.com/charmbracelet/bubbletea
cp -R "$($GO list -m -f '{{.Dir}}' github.com/charmbracelet/bubbletea)" "$WORK/bubbletea"
chmod -R u+w "$WORK/bubbletea"
cp web/_bubbletea/*.go "$WORK/bubbletea/"
cp go.mod go.sum "$WORK/"
$GO mod edit -replace "github.com/charmbracelet/bubbletea=$WORK/bubbletea" "$WORK/go.mod"

rm -rf "$OUT"
mkdir -p "$OUT/vendor"
version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)
GOOS=js GOARCH=wasm $GO build -modfile="$WORK/go.mod" -trimpath \
	-ldflags "-s -w -X main.version=$version" -o "$OUT/gag.wasm" ./cmd/gag
# The glue has to come from the same Go that built the wasm.
cp "$($GO env GOROOT)/lib/wasm/wasm_exec.js" "$OUT/vendor/"

# xterm.js, pinned.
mkdir "$WORK/npm"
(cd "$WORK/npm" && npm pack --silent "@xterm/xterm@$XTERM" "@xterm/addon-fit@$XTERM_FIT" \
	"@xterm/addon-webgl@$XTERM_WEBGL" >/dev/null)
for pkg in xterm addon-fit addon-webgl; do
	mkdir "$WORK/npm/$pkg"
	tar -xzf "$WORK/npm/xterm-$pkg-"*.tgz -C "$WORK/npm/$pkg"
	cp "$WORK/npm/$pkg/package/lib/$pkg.js" "$OUT/vendor/"
done
cp "$WORK/npm/xterm/package/css/xterm.css" "$OUT/vendor/"
cp "$WORK/npm/xterm/package/LICENSE" "$OUT/vendor/xterm.LICENSE"

cp web/index.html "$OUT/"
echo "gag $version -> $OUT ($(du -sh "$OUT" | cut -f1))"
