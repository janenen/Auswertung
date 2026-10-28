#!/usr/bin/env bash
# Builds the release archives for all platforms into dist/.
# Usage: scripts/build-release.sh <version>
# The frontend must be built first (cd web && npm ci && npm run build).
# Everything is cross-compiled without CGO, so one machine builds all targets.
set -euo pipefail

version=${1:?usage: $0 <version>}
targets=(
  windows/amd64
  windows/arm64
  linux/amd64
  linux/arm64
  darwin/amd64
  darwin/arm64
)

cd "$(dirname "$0")/.."
if [ ! -f web/dist/index.html ]; then
  echo "web/dist/index.html missing: build the frontend first" >&2
  exit 1
fi

rm -rf dist
mkdir -p dist
for target in "${targets[@]}"; do
  os=${target%/*}
  arch=${target#*/}
  name="auswertung-${version}-${os}-${arch}"
  dir="dist/${name}"
  exe=auswertung
  [ "$os" = windows ] && exe=auswertung.exe

  echo "building ${name}"
  mkdir -p "$dir"
  CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath \
    -ldflags "-s -w -X main.version=${version}" -o "${dir}/${exe}" ./cmd/auswertung
  cp README.md LICENSE "$dir/"

  # Windows users expect zip files; tar keeps the executable bit elsewhere.
  if [ "$os" = windows ]; then
    if command -v zip > /dev/null; then
      (cd dist && zip -qr "${name}.zip" "$name")
    else
      (cd dist && bsdtar -a -cf "${name}.zip" "$name")
    fi
  else
    # Set the modes explicitly: file systems like NTFS do not keep them.
    tar -C dist --mode=0644 -cf "dist/${name}.tar" "${name}/README.md" "${name}/LICENSE"
    tar -C dist --mode=0755 -rf "dist/${name}.tar" "${name}/${exe}"
    gzip -9 "dist/${name}.tar"
  fi
  rm -rf "$dir"
done

(cd dist && sha256sum auswertung-* > SHA256SUMS)
ls -l dist
