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
DEB="$OUT/rag-studio_${VERSION_NUM}_${ARCH}.deb"
dpkg-deb --build --root-owner-group "$ROOT" "$DEB"

if [ -n "${GPG_PRIVATE_KEY:-}" ]; then
  export GNUPGHOME
  GNUPGHOME="$(mktemp -d)"
  chmod 700 "$GNUPGHOME"
  printf '%s\n' "$GPG_PRIVATE_KEY" | gpg --batch --yes --import
  KEY_ID="${GPG_KEY_ID:-863320B6B89B26B2988B55883D8DC7AC7D242CF0}"
  # CI has no TTY: force loopback pinentry with an empty passphrase.
  : > "$GNUPGHOME/passphrase"
  GPG_OPTS="--pinentry-mode loopback --passphrase-file $GNUPGHOME/passphrase --batch --yes"
  printf 'allow-loopback-pinentry\n' > "$GNUPGHOME/gpg-agent.conf"
  gpgconf --kill gpg-agent 2>/dev/null || true
  dpkg-sig --sign builder -k "$KEY_ID" -g "$GPG_OPTS" "$DEB"
  gpg --armor --export "$KEY_ID" > "$OUT/rag-studio-release.gpg.key"
  gpg $GPG_OPTS --armor --detach-sign --output "$DEB.asc" "$DEB"
fi

echo "$DEB"
