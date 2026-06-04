#!/bin/sh
# install.sh – Install the ric CLI
#
# Usage:
#   curl -fsSL https://railsiran.org/install.sh | sh
#
# Override the version:
#   curl -fsSL https://railsiran.org/install.sh | RIC_VERSION=1.0.0 sh
#
# Override the download host (for self-hosted mirrors):
#   curl -fsSL https://railsiran.org/install.sh | RIC_BASE_URL=https://mirror.example.com sh

set -e

# --- Configuration ----------------------------------------------------------
BASE_URL="${RIC_BASE_URL:-https://railsiran.org}"
VERSION="${RIC_VERSION:-latest}"

# --- Color output helpers (only when stdout is a terminal) ------------------
if [ -t 1 ]; then
  bold()   { printf "\033[1m%s\033[0m\n" "$*"; }
  green()  { printf "\033[32m%s\033[0m\n" "$*"; }
  red()    { printf "\033[31m%s\033[0m\n" "$*" >&2; }
  yellow() { printf "\033[33m%s\033[0m\n" "$*"; }
else
  bold()   { printf "%s\n" "$*"; }
  green()  { printf "%s\n" "$*"; }
  red()    { printf "%s\n" "$*" >&2; }
  yellow() { printf "%s\n" "$*"; }
fi

# --- Sanity: curl is the only hard dependency -------------------------------
if ! command -v curl >/dev/null 2>&1; then
  red "curl is required but not found on PATH."
  red "Install curl and re-run this script."
  exit 1
fi

# --- Detect OS and architecture ---------------------------------------------
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$OS" in
  linux)  os="linux"  ;;
  darwin) os="darwin" ;;
  *)
    red "Unsupported OS: $OS"
    red "ric supports Linux and macOS via this installer."
    red "For Windows, download ric-windows-amd64.exe from ${BASE_URL}/builds/${VERSION}/windows-amd64.exe"
    exit 1
    ;;
esac

case "$ARCH" in
  x86_64|amd64)  arch="amd64" ;;
  aarch64|arm64) arch="arm64" ;;
  *)
    red "Unsupported architecture: $ARCH"
    red "ric ships builds for amd64 and arm64 only."
    exit 1
    ;;
esac

platform="${os}-${arch}"
url="${BASE_URL}/builds/${VERSION}/${platform}"

# --- Announce ---------------------------------------------------------------
bold "Installing ric CLI"
echo "  Platform: ${platform}"
echo "  Version:  ${VERSION}"
echo "  URL:      ${url}"
echo ""

# --- Check the URL exists before committing to a download -------------------
if ! curl -fsSL --head "${url}" >/dev/null 2>&1; then
  red "Build not found at: ${url}"
  yellow ""
  yellow "Available platforms at version '${VERSION}':"
  for p in linux-amd64 linux-arm64 darwin-arm64 windows-amd64.exe; do
    echo "  ${BASE_URL}/builds/${VERSION}/${p}"
  done
  yellow ""
  yellow "To install a specific version:"
  echo "  curl -fsSL ${BASE_URL}/install.sh | RIC_VERSION=1.0.0 sh"
  exit 1
fi

# --- Download to a temp file, clean up on any exit --------------------------
tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT

echo "Downloading..."
curl -fsSL "${url}" -o "$tmp"
chmod +x "$tmp"

# --- Verify the binary actually runs ----------------------------------------
echo ""
if ! "$tmp" version; then
  red ""
  red "The downloaded binary failed to run."
  red "The download may be corrupted, or it doesn't match your platform."
  exit 1
fi
echo ""

# --- Choose install location ------------------------------------------------
if [ -w /usr/local/bin ]; then
  dest="/usr/local/bin/ric"
  echo "Installing to /usr/local/bin (system-wide)..."
else
  mkdir -p "${HOME}/.local/bin"
  dest="${HOME}/.local/bin/ric"
  echo "Installing to ~/.local/bin (user)..."
fi

mv "$tmp" "$dest"
trap - EXIT   # tmp is now consumed; don't try to remove it on exit

# --- Final PATH check + shell guidance --------------------------------------
dir=$(dirname "$dest")
case ":$PATH:" in
  *":$dir:"*)
    green "ric installed successfully at $dest"
    ;;
  *)
    green "ric installed at $dest"
    yellow ""
    yellow "$dir is not on your PATH. Add it with:"
    echo ""
    case "$SHELL" in
      */zsh)
        echo "  echo 'export PATH=\"$dir:\$PATH\"' >> ~/.zshrc"
        echo "  source ~/.zshrc"
        ;;
      */bash)
        echo "  echo 'export PATH=\"$dir:\$PATH\"' >> ~/.bashrc"
        echo "  source ~/.bashrc"
        ;;
      */fish)
        echo "  fish_add_path $dir"
        ;;
      *)
        echo "  export PATH=\"$dir:\$PATH\""
        ;;
    esac
    ;;
esac

echo ""
echo "Run 'ric help' to get started."
