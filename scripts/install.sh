#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# logwatch quick-install script
# Usage: curl -sSL https://raw.githubusercontent.com/cielavenir/logwatch/main/scripts/install.sh | bash
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

VERSION="${VERSION:-$(curl -sSL https://api.github.com/repos/cielavenir/logwatch/releases/latest 2>/dev/null | grep '"tag_name"' | sed 's/.*"v\?\([^"]*\)".*/\1/' || echo '0.3.0')}"
INSTALL_PREFIX="${INSTALL_PREFIX:-/usr/local/bin}"
FORCE="${FORCE:-}"

# Detect OS and architecture.
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  armv7l) ARCH="arm7" ;;
  i386|i686) ARCH="386" ;;
esac

ARTIFACT="logwatch-${OS}-${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/cielavenir/logwatch/releases/latest/download/${ARTIFACT}"
INSTALL_PATH="${INSTALL_PREFIX}/logwatch"

# Check for existing installation.
if [[ -f "$INSTALL_PATH" && "$FORCE" != "1" ]]; then
  echo "logwatch is already installed at ${INSTALL_PATH}. Run with FORCE=1 to overwrite."
  exit 1
fi

echo "Downloading logwatch ${VERSION} for ${OS}/${ARCH}…"
TMPDIR=$(mktemp -d)
trap "rm -rf $TMPDIR" EXIT

curl -sSL --fail -o "${TMPDIR}/${ARTIFACT}" "${DOWNLOAD_URL}"
tar -xzf "${TMPDIR}/${ARTIFACT}" -C "$TMPDIR"
chmod +x "${TMPDIR}/logwatch"

echo "Installing to ${INSTALL_PATH}…"
sudo install -Dm755 "${TMPDIR}/logwatch" "$INSTALL_PATH"

echo "Installed logwatch ${VERSION} to ${INSTALL_PATH}"
logwatch --version
