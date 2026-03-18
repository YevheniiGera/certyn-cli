#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Install certyn CLI.

Usage:
  install.sh [options]

Options:
  --version <tag>       Release tag to install (for example: v0.1.0)
  --latest              Install latest release tag from GitHub API
  --dir <path>          Install directory (default: ~/.local/bin)
  --repo <owner/name>   GitHub repo (default: YevheniiGera/certyn-cli)
  --base-url <url>      Release base URL override
  --skip-verify         Skip SHA256 verification (not recommended)
  -h, --help            Show help

Environment overrides:
  CERTYN_INSTALL_VERSION
  CERTYN_INSTALL_DIR
  CERTYN_INSTALL_REPO
  CERTYN_INSTALL_BASE_URL
EOF
}

log() {
  printf '%s\n' "$*" >&2
}

fail() {
  log "error: $*"
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

compute_sha256() {
  local file="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$file" | awk '{print $1}'
    return
  fi
  if command -v openssl >/dev/null 2>&1; then
    openssl dgst -sha256 "$file" | awk '{print $NF}'
    return
  fi
  fail "no SHA256 tool found (need sha256sum, shasum, or openssl)"
}

resolve_latest_version() {
  local repo="$1"
  local api_url="https://api.github.com/repos/${repo}/releases/latest"
  local tag
  tag="$(curl -fsSL "$api_url" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
  [[ -n "$tag" ]] || fail "failed to resolve latest release tag from ${api_url}"
  printf '%s' "$tag"
}

detect_os() {
  case "$(uname -s)" in
    Linux) printf 'linux' ;;
    Darwin) printf 'darwin' ;;
    *) fail "unsupported OS: $(uname -s)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) printf 'amd64' ;;
    arm64|aarch64) printf 'arm64' ;;
    *) fail "unsupported architecture: $(uname -m)" ;;
  esac
}

VERSION="${CERTYN_INSTALL_VERSION:-}"
INSTALL_DIR="${CERTYN_INSTALL_DIR:-$HOME/.local/bin}"
REPO="${CERTYN_INSTALL_REPO:-YevheniiGera/certyn-cli}"
BASE_URL="${CERTYN_INSTALL_BASE_URL:-}"
SKIP_VERIFY=0
USE_LATEST=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)
      [[ $# -ge 2 ]] || fail "--version requires a value"
      VERSION="$2"
      shift 2
      ;;
    --latest)
      USE_LATEST=1
      shift
      ;;
    --dir)
      [[ $# -ge 2 ]] || fail "--dir requires a value"
      INSTALL_DIR="$2"
      shift 2
      ;;
    --repo)
      [[ $# -ge 2 ]] || fail "--repo requires a value"
      REPO="$2"
      shift 2
      ;;
    --base-url)
      [[ $# -ge 2 ]] || fail "--base-url requires a value"
      BASE_URL="$2"
      shift 2
      ;;
    --skip-verify)
      SKIP_VERIFY=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "unknown argument: $1"
      ;;
  esac
done

require_cmd curl
require_cmd tar

if [[ -z "$BASE_URL" ]]; then
  BASE_URL="https://github.com/${REPO}/releases/download"
fi
BASE_URL="${BASE_URL%/}"

if [[ -z "$VERSION" ]]; then
  if [[ "$USE_LATEST" -eq 0 ]]; then
    log "no --version provided; resolving latest release. Use --version for pinned installs."
  fi
  VERSION="$(resolve_latest_version "$REPO")"
fi

OS="$(detect_os)"
ARCH="$(detect_arch)"
ASSET="certyn_${OS}_${ARCH}.tar.gz"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

ARCHIVE_PATH="${TMP_DIR}/${ASSET}"
CHECKSUM_PATH="${ARCHIVE_PATH}.sha256"
ARCHIVE_URL="${BASE_URL}/${VERSION}/${ASSET}"
CHECKSUM_URL="${ARCHIVE_URL}.sha256"

log "Downloading ${ARCHIVE_URL}"
curl -fL --retry 3 --retry-delay 1 -o "$ARCHIVE_PATH" "$ARCHIVE_URL"

if [[ "$SKIP_VERIFY" -eq 0 ]]; then
  log "Downloading ${CHECKSUM_URL}"
  curl -fL --retry 3 --retry-delay 1 -o "$CHECKSUM_PATH" "$CHECKSUM_URL"

  EXPECTED_SHA="$(awk '{print $1}' "$CHECKSUM_PATH" | tr -d '\r\n')"
  [[ -n "$EXPECTED_SHA" ]] || fail "unable to read expected SHA256 from checksum file"
  ACTUAL_SHA="$(compute_sha256 "$ARCHIVE_PATH" | tr -d '\r\n')"

  if [[ "${ACTUAL_SHA}" != "${EXPECTED_SHA}" ]]; then
    fail "checksum mismatch for ${ASSET} (expected ${EXPECTED_SHA}, got ${ACTUAL_SHA})"
  fi
fi

tar -xzf "$ARCHIVE_PATH" -C "$TMP_DIR"

BINARY_PATH="${TMP_DIR}/certyn"
if [[ ! -f "$BINARY_PATH" ]]; then
  BINARY_PATH="$(find "$TMP_DIR" -maxdepth 3 -type f -name 'certyn' | head -n1 || true)"
fi
[[ -n "$BINARY_PATH" && -f "$BINARY_PATH" ]] || fail "certyn binary not found in archive"

mkdir -p "$INSTALL_DIR"
if command -v install >/dev/null 2>&1; then
  install -m 0755 "$BINARY_PATH" "${INSTALL_DIR}/certyn"
else
  cp "$BINARY_PATH" "${INSTALL_DIR}/certyn"
  chmod 0755 "${INSTALL_DIR}/certyn"
fi

log "Installed certyn to ${INSTALL_DIR}/certyn"
if "${INSTALL_DIR}/certyn" --version >/dev/null 2>&1; then
  "${INSTALL_DIR}/certyn" --version
fi
log ""
log "Next steps:"
log "  Local machine: ${INSTALL_DIR}/certyn login"
log "  CI/agents: set CERTYN_API_KEY and run ${INSTALL_DIR}/certyn ci run ..."

# Install Claude Code skill (if skill files are in the archive)
SKILL_SRC="$(find "$TMP_DIR" -maxdepth 3 -type d -name 'skills' | head -n1 || true)"
if [[ -n "$SKILL_SRC" && -d "$SKILL_SRC/certyn" ]]; then
  CLAUDE_SKILLS_DIR="${HOME}/.claude/skills/certyn"
  mkdir -p "$CLAUDE_SKILLS_DIR"
  cp "$SKILL_SRC/certyn/SKILL.md" "$CLAUDE_SKILLS_DIR/SKILL.md"
  log "Installed Claude Code skill to ${CLAUDE_SKILLS_DIR}"
fi

if [[ ":${PATH}:" != *":${INSTALL_DIR}:"* ]]; then
  log ""
  log "Add certyn to your PATH:"
  log "  export PATH=\"${INSTALL_DIR}:\$PATH\""
fi
