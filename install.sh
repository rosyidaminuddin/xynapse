#!/usr/bin/env bash
#
# Install xynapse as a global command.
#
# - Builds the binary into the repo's bin/ directory
# - Copies it to a directory on PATH (default: ~/.local/bin)
# - Installs config/config.yaml to ~/.config/xynapse/config.yaml (if missing)
#
# Usage:
#   ./install.sh              # install to ~/.local/bin (recommended)
#   PREFIX=/usr/local ./install.sh   # install to /usr/local/bin (requires sudo)
#   DESTDIR=/opt/xynapse ./install.sh
#
# Environment overrides:
#   PREFIX    base directory containing bin/ (default: $HOME/.local)
#   DESTDIR   install everything under an alternate root
#
set -euo pipefail

cd "$(dirname "$0")"

BIN_NAME="xynapse"
CONFIG_SRC="config/config.yaml"
CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/xynapse"

DESTDIR="${DESTDIR:-}"
PREFIX="${PREFIX:-$HOME/.local}"
BIN_DIR="$DESTDIR$PREFIX/bin"

echo "==> Building $BIN_NAME"
go build -o "bin/$BIN_NAME" .

echo "==> Installing $BIN_NAME to $BIN_DIR"
mkdir -p "$BIN_DIR"
install -m 0755 "bin/$BIN_NAME" "$BIN_DIR/$BIN_NAME"

CONFIG_DEST="$DESTDIR$CONFIG_DIR"
if [[ ! -f "$CONFIG_DEST/config.yaml" ]]; then
  echo "==> Installing config to $CONFIG_DEST/config.yaml"
  mkdir -p "$CONFIG_DEST"
  install -m 0644 "$CONFIG_SRC" "$CONFIG_DEST/config.yaml"
  echo "    Edit $CONFIG_DEST/config.yaml to set your Jira credentials."
else
  echo "==> Config already exists at $CONFIG_DEST/config.yaml, leaving it untouched"
fi

echo
echo "Done. Run '$BIN_NAME --help' to verify."
echo "Add to your shell rc if $BIN_DIR is not already on PATH:"
echo "  export PATH=\"$BIN_DIR:\$PATH\""
