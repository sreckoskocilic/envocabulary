#!/usr/bin/env sh
# envocabulary installer
# Usage: curl -fsSL https://raw.githubusercontent.com/sreckoskocilic/envocabulary/main/install.sh | sh
#        curl -fsSL https://raw.githubusercontent.com/sreckoskocilic/envocabulary/main/install.sh | sh -s -- --version v0.1.0
#        curl -fsSL https://raw.githubusercontent.com/sreckoskocilic/envocabulary/main/install.sh | sh -s -- --bin-dir /usr/local/bin

set -eu

REPO="sreckoskocilic/envocabulary"
BIN_NAME="envocabulary"
VERSION=""
BIN_DIR=""

usage() {
    cat <<EOF
envocabulary installer

Usage:
  install.sh [--version VERSION] [--bin-dir DIR]

Options:
  --version VERSION   install a specific version (e.g. v0.1.0). Default: latest release.
  --bin-dir DIR       install destination. Default: \$HOME/.local/bin (no sudo) or /usr/local/bin if writable.
  -h, --help          show this help and exit.
EOF
}

while [ $# -gt 0 ]; do
    case "$1" in
        --version)
            VERSION="$2"; shift 2 ;;
        --bin-dir)
            BIN_DIR="$2"; shift 2 ;;
        -h|--help)
            usage; exit 0 ;;
        *)
            echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
    esac
done

# --- detect OS / arch ----------------------------------------------------------

uname_os() {
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    case "$os" in
        darwin) echo "darwin" ;;
        linux)  echo "linux" ;;
        freebsd) echo "freebsd" ;;
        *) echo "unsupported OS: $os" >&2; exit 1 ;;
    esac
}

uname_arch() {
    arch=$(uname -m)
    case "$arch" in
        x86_64|amd64)  echo "amd64" ;;
        arm64|aarch64) echo "arm64" ;;
        i386|i686)     echo "386" ;;
        armv7l)        echo "armv7" ;;
        *) echo "unsupported architecture: $arch" >&2; exit 1 ;;
    esac
}

OS=$(uname_os)
ARCH=$(uname_arch)

# --- pick destination ----------------------------------------------------------

pick_bin_dir() {
    if [ -n "$BIN_DIR" ]; then
        echo "$BIN_DIR"
        return
    fi
    if [ -w "/usr/local/bin" ]; then
        echo "/usr/local/bin"
        return
    fi
    mkdir -p "$HOME/.local/bin"
    echo "$HOME/.local/bin"
}

DEST_DIR=$(pick_bin_dir)
DEST="$DEST_DIR/$BIN_NAME"

# --- resolve version -----------------------------------------------------------

resolve_version() {
    if [ -n "$VERSION" ]; then
        echo "$VERSION"
        return
    fi
    # GitHub redirects /releases/latest to /releases/tag/<version>
    curl -fsSI "https://github.com/$REPO/releases/latest" \
        | awk -F/ '/^location:|^Location:/ {gsub(/[\r\n]/,"",$NF); print $NF}' \
        | tail -n1
}

VERSION=$(resolve_version)
if [ -z "$VERSION" ]; then
    echo "could not determine latest version; specify --version" >&2
    exit 1
fi
VER_NO_V=${VERSION#v}

# --- download & install --------------------------------------------------------

ARCHIVE="${BIN_NAME}_${VER_NO_V}_${OS}_${ARCH}.tar.gz"
CHECKSUMS="${BIN_NAME}_${VER_NO_V}_checksums.txt"
BASE_URL="https://github.com/$REPO/releases/download/$VERSION"
URL="$BASE_URL/$ARCHIVE"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

echo "Downloading $URL"
if ! curl -fsSL "$URL" -o "$TMP/$ARCHIVE"; then
    echo "download failed; check that release $VERSION exists for $OS/$ARCH" >&2
    exit 1
fi

echo "Verifying checksum..."
if ! curl -fsSL "$BASE_URL/$CHECKSUMS" -o "$TMP/$CHECKSUMS"; then
    echo "checksum file download failed; cannot verify integrity" >&2
    exit 1
fi

EXPECTED=$(grep "$ARCHIVE" "$TMP/$CHECKSUMS" | awk '{print $1}')
if [ -z "$EXPECTED" ]; then
    echo "no checksum found for $ARCHIVE in $CHECKSUMS" >&2
    exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
    ACTUAL=$(sha256sum "$TMP/$ARCHIVE" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
    ACTUAL=$(shasum -a 256 "$TMP/$ARCHIVE" | awk '{print $1}')
else
    echo "no sha256sum or shasum available; cannot verify checksum" >&2
    exit 1
fi

if [ "$ACTUAL" != "$EXPECTED" ]; then
    echo "checksum mismatch: expected $EXPECTED, got $ACTUAL" >&2
    exit 1
fi

# --- optional cosign signature verification -----------------------------------

if command -v cosign >/dev/null 2>&1; then
    SIG_URL="$BASE_URL/$CHECKSUMS.sig"
    CERT_URL="$BASE_URL/$CHECKSUMS.pem"
    if curl -fsSL "$SIG_URL" -o "$TMP/$CHECKSUMS.sig" 2>/dev/null && \
       curl -fsSL "$CERT_URL" -o "$TMP/$CHECKSUMS.pem" 2>/dev/null; then
        echo "Verifying cosign signature..."
        if cosign verify-blob \
            --certificate "$TMP/$CHECKSUMS.pem" \
            --signature "$TMP/$CHECKSUMS.sig" \
            --certificate-identity-regexp "https://github.com/$REPO" \
            --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
            "$TMP/$CHECKSUMS" >/dev/null 2>&1; then
            echo "✓ Cosign signature verified"
        else
            echo "✗ cosign signature verification failed" >&2
            exit 1
        fi
    else
        echo "  (cosign signature files not found in release; skipping signature check)"
    fi
else
    echo "  (cosign not installed; skipping signature verification)"
fi

tar -xzf "$TMP/$ARCHIVE" -C "$TMP"

if [ ! -f "$TMP/$BIN_NAME" ]; then
    echo "binary not found in archive" >&2
    exit 1
fi

install -m 0755 "$TMP/$BIN_NAME" "$DEST"

# macOS: remove quarantine attribute so Gatekeeper doesn't block first run
if [ "$OS" = "darwin" ] && command -v xattr >/dev/null 2>&1; then
    xattr -d com.apple.quarantine "$DEST" 2>/dev/null || true
fi

echo ""
echo "✓ Installed $BIN_NAME $VERSION to $DEST"

# Warn if dest isn't on PATH
case ":$PATH:" in
    *":$DEST_DIR:"*) ;;
    *)
        echo ""
        echo "⚠ $DEST_DIR is not on your \$PATH."
        echo "  Add this to your shell config:"
        echo "    export PATH=\"$DEST_DIR:\$PATH\""
        ;;
esac

echo ""
"$DEST" --version
