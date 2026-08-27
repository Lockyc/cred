#!/usr/bin/env bash
# install.sh — installs/updates the cred binary via `go install`.
# Usage:  bash install.sh
#    or:  curl -fsSL https://raw.githubusercontent.com/lockyc/cred/main/install.sh | bash
#
# The curl URL requires the GitHub repo to be PUBLIC (cred is).
#
# cred is a Go CLI, not a bundle — this installs a BINARY to the Go bin dir;
# there is no ~/.cred clone to manage and no config to seed.
set -e

MODULE="github.com/lockyc/cred"

command -v go >/dev/null 2>&1 || {
  echo "cred: Go is required (https://go.dev/dl/). Install it, then re-run." >&2
  exit 1
}

# IN_REPO when run from a checkout: build the current tree so uncommitted
# work is installed. NOT_IN_REPO: install the published module at @latest.
if [ -f go.mod ] && grep -q "^module $MODULE\$" go.mod 2>/dev/null && [ -f main.go ]; then
  echo "Installing cred from the current checkout (go install .) ..."
  go install .
else
  echo "Installing cred from $MODULE@latest ..."
  go install "$MODULE@latest"
fi

BIN_DIR="$(go env GOBIN)"
[ -n "$BIN_DIR" ] || BIN_DIR="$(go env GOPATH)/bin"
[ -n "$BIN_DIR" ] || BIN_DIR="$HOME/go/bin"

# Exec the freshly installed binary once. This is load-bearing, not
# cosmetic: a freshly written binary has a new code-signature cache miss, and
# on macOS the first exec blocks in dyld for a live Gatekeeper assessment
# (~1s). Absorbing that here, while the user is already waiting on the
# install, is cheaper than paying it on their first real `cred set`.
VERSION="$("$BIN_DIR/cred" version 2>/dev/null || echo "")"

echo ""
echo "cred${VERSION:+ v$VERSION} installed → $BIN_DIR/cred"
case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) echo "  warning — $BIN_DIR is not on your PATH; add it so 'cred' resolves." >&2 ;;
esac
echo ""
echo "Try it: cred set ~/.config/example/token"
