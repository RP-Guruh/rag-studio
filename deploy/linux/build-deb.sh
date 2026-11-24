#!/usr/bin/env bash
set -euo pipefail

# Usage: build-deb.sh <binary> <version-tag> <arch> <output-dir>
BIN="${1:?binary path required}"
VERSION="${2:?version required}"
ARCH="${3:?architecture required}"
OUT="${4:?output dir required}"

VERSION_NUM="${VERSION#v}"
ROOT="$(mktemp -d)"
trap 'rm -rf "$ROOT"' EXIT

mkdir -p "$ROOT/DEBIAN" "$ROOT/usr/bin" "$ROOT/usr/share/rag-studio/deploy/local" "$ROOT/usr/share/doc/rag-studio"
install -m 755 "$BIN" "$ROOT/usr/bin/rag-studio"
cp deploy/local/docker-compose.yml "$ROOT/usr/share/rag-studio/deploy/local/docker-compose.yml"
cp README.md "$ROOT/usr/share/doc/rag-studio/README.md"

cat > "$ROOT/DEBIAN/control" <<EOF
Package: rag-studio
Version: ${VERSION_NUM}
Section: utils
Priority: optional
Architecture: ${ARCH}
Depends: libgtk-4-1, libwebkitgtk-6.0-4
Maintainer: RP-Guruh <noreply@github.com>
Homepage: https://github.com/RP-Guruh/rag-studio
Description: Desktop RAG Studio for knowledge bases and personas
 RAG Studio is a desktop application for building isolated RAG projects
 with their own documents, personas, and vector knowledge bases.
EOF

mkdir -p "$OUT"
dpkg-deb --build --root-owner-group "$ROOT" "$OUT/rag-studio_${VERSION_NUM}_${ARCH}.deb"
echo "$OUT/rag-studio_${VERSION_NUM}_${ARCH}.deb"
