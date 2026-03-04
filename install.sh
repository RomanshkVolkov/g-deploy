#!/bin/sh
set -e

REPO="RomanshkVolkov/g-deploy"
BINARY="g-deploy"

# ── Colours ──────────────────────────────────────────────────────────────────
green="\033[1;32m" yellow="\033[1;33m" red="\033[1;31m" reset="\033[0m"
info()    { printf "${yellow}[*]${reset} %s\n" "$*"; }
success() { printf "${green}[✓]${reset} %s\n" "$*"; }
error()   { printf "${red}[x]${reset} %s\n" "$*" >&2; exit 1; }

# ── Detect install dir ───────────────────────────────────────────────────────
if [ "$(id -u)" = "0" ]; then
  INSTALL_DIR="/usr/local/bin"
else
  INSTALL_DIR="${HOME}/.local/bin"
  mkdir -p "$INSTALL_DIR"
fi

# ── Detect OS ────────────────────────────────────────────────────────────────
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  linux|darwin) ;;
  *) error "Unsupported OS: $OS" ;;
esac

# ── Detect architecture ──────────────────────────────────────────────────────
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)          ARCH="amd64" ;;
  aarch64|arm64)   ARCH="arm64" ;;
  *)               error "Unsupported architecture: $ARCH" ;;
esac

# ── Resolve latest release from GitHub API ───────────────────────────────────
info "Checking latest release..."
LATEST=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' \
  | sed 's/.*"tag_name": *"\(.*\)".*/\1/')

[ -z "$LATEST" ] && error "Could not fetch latest release. Check your connection."

# ── Check currently installed version ────────────────────────────────────────
CURRENT=""
if command -v "$BINARY" > /dev/null 2>&1; then
  CURRENT=$("$BINARY" version 2>/dev/null | grep -o 'v[0-9]*\.[0-9]*\.[0-9]*' || true)
fi

if [ "$CURRENT" = "$LATEST" ]; then
  success "Already up to date: $CURRENT"
  exit 0
fi

if [ -n "$CURRENT" ]; then
  info "Updating $BINARY: $CURRENT → $LATEST"
else
  info "Installing $BINARY $LATEST"
fi

# ── Download binary ───────────────────────────────────────────────────────────
ASSET="${BINARY}-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/download/${LATEST}/${ASSET}"

info "Downloading ${URL}..."
TMP=$(mktemp)
curl -fsSL "$URL" -o "$TMP" || error "Download failed. Check that release ${LATEST} has asset: ${ASSET}"
chmod +x "$TMP"

# ── Install ───────────────────────────────────────────────────────────────────
mv "$TMP" "${INSTALL_DIR}/${BINARY}"
success "Installed ${BINARY} ${LATEST} → ${INSTALL_DIR}/${BINARY}"

# ── PATH hint ─────────────────────────────────────────────────────────────────
case ":$PATH:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    printf "\n${yellow}Add to your PATH:${reset}\n"
    printf "  export PATH=\"\$PATH:${INSTALL_DIR}\"\n\n"
    ;;
esac
