#!/bin/sh
# Install gag (Git a Garden) from the latest GitHub release.
#
#   curl -fsSL https://raw.githubusercontent.com/RursusAeternum/GitAGarden/main/install.sh | sh
#
# Env overrides:
#   GAG_VERSION      release tag to install, e.g. v0.1.0 (default: latest)
#   GAG_INSTALL_DIR  where to put the binary (default: /usr/local/bin if
#                    writable, else ~/.local/bin)
set -eu

REPO="RursusAeternum/GitAGarden"

say() { printf 'gag-install: %s\n' "$*" >&2; }
die() { say "$*"; exit 1; }

need() { command -v "$1" >/dev/null 2>&1 || die "needs '$1'"; }
need curl
need tar
need uname

case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) die "unsupported OS: $(uname -s)" ;;
esac

case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  armv7*) arch=armv7 ;;
  armv6*) arch=armv6 ;;
  *) die "unsupported CPU: $(uname -m)" ;;
esac
[ "$os" = darwin ] && [ "${arch#armv}" != "$arch" ] && die "unsupported: darwin/$arch"

asset="gag_${os}_${arch}.tar.gz"
if [ -n "${GAG_VERSION:-}" ]; then
  base="https://github.com/$REPO/releases/download/$GAG_VERSION"
else
  base="https://github.com/$REPO/releases/latest/download"
fi

if [ -n "${GAG_INSTALL_DIR:-}" ]; then
  dir=$GAG_INSTALL_DIR
elif [ -w /usr/local/bin ]; then
  dir=/usr/local/bin
else
  dir="$HOME/.local/bin"
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

say "downloading $asset"
curl -fsSL "$base/$asset" -o "$tmp/$asset" || die "download failed: $base/$asset"
curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt" || die "download failed: checksums.txt"

expected=$(grep " $asset\$" "$tmp/checksums.txt" | cut -d' ' -f1)
[ -n "$expected" ] || die "no checksum listed for $asset"
if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$tmp/$asset" | cut -d' ' -f1)
else
  actual=$(shasum -a 256 "$tmp/$asset" | cut -d' ' -f1)
fi
[ "$expected" = "$actual" ] || die "checksum mismatch for $asset"

tar -xzf "$tmp/$asset" -C "$tmp" gag
mkdir -p "$dir"
mv "$tmp/gag" "$dir/gag"
chmod +x "$dir/gag"
say "installed $("$dir/gag" version) to $dir/gag"

case ":$PATH:" in
  *":$dir:"*) ;;
  *) say "note: $dir is not on your PATH; add it, e.g. export PATH=\"$dir:\$PATH\"" ;;
esac
