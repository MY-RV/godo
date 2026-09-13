#!/usr/bin/env bash
# Build release-like artifacts locally (no GitHub upload).
# Usage:
#   ./scripts/release-local.sh
#   VERSION=v0.1.0 ./scripts/release-local.sh
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

VERSION="${VERSION:-}"
if [[ -z "$VERSION" ]]; then
  if VERSION=$(git describe --tags --exact-match 2>/dev/null); then
    :
  elif VERSION=$(git describe --tags --always --dirty 2>/dev/null); then
    VERSION="0.1.0-dev+${VERSION}"
  else
    VERSION="0.1.0-dev"
  fi
fi
VERSION="${VERSION#v}"
DIST="${DIST:-dist}"
rm -rf "$DIST"
mkdir -p "$DIST"

LDFLAGS="-X github.com/my-rv/godo.Version=v${VERSION}"
echo "version → v${VERSION}"
echo "ldflags → ${LDFLAGS}"

targets=(
  "darwin arm64"
  "darwin amd64"
  "linux amd64"
  "linux arm64"
)

for t in "${targets[@]}"; do
  # shellcheck disable=SC2086
  set -- $t
  os=$1
  arch=$2
  out="${DIST}/godo_${VERSION}_${os}_${arch}"
  echo "build ${out}"
  GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o "$out" ./cmd/godo
done

# Host binary for dogfood / PATH
host_out="${DIST}/godo_${VERSION}_$(go env GOOS)_$(go env GOARCH)"
cp "$host_out" "${DIST}/godo"
chmod +x "${DIST}/godo"

(
  cd "$DIST"
  if command -v shasum >/dev/null; then
    shasum -a 256 godo_* > SHA256SUMS
  else
    sha256sum godo_* > SHA256SUMS
  fi
)

echo
echo "artifacts in ${DIST}/:"
ls -la "$DIST"
echo
"${DIST}/godo" --version
echo "checksums:"
cat "${DIST}/SHA256SUMS"
