#!/bin/sh

set -eu

OWNER="jonaswide"
REPO="intervals-cli"
BINARY="intervals"
VERSION="${VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-}"

fail() {
  printf '%s\n' "error: $*" >&2
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1
}

download() {
  url="$1"
  output="$2"
  if need_cmd curl; then
    curl -fsSL "$url" -o "$output"
    return
  fi
  if need_cmd wget; then
    wget -qO "$output" "$url"
    return
  fi
  fail "curl or wget is required"
}

detect_os() {
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$os" in
    darwin|linux)
      printf '%s' "$os"
      ;;
    *)
      fail "unsupported operating system: $os"
      ;;
  esac
}

detect_arch() {
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64)
      printf '%s' "amd64"
      ;;
    arm64|aarch64)
      printf '%s' "arm64"
      ;;
    *)
      fail "unsupported architecture: $arch"
      ;;
  esac
}

default_install_dir() {
  if [ -d "/usr/local/bin" ] && [ -w "/usr/local/bin" ]; then
    printf '%s' "/usr/local/bin"
    return
  fi
  printf '%s' "$HOME/.local/bin"
}

checksum_file() {
  file="$1"
  if need_cmd shasum; then
    shasum -a 256 "$file" | awk '{print $1}'
    return
  fi
  if need_cmd sha256sum; then
    sha256sum "$file" | awk '{print $1}'
    return
  fi
  fail "shasum or sha256sum is required"
}

resolve_urls() {
  asset="$1"
  if [ "$VERSION" = "latest" ]; then
    archive_url="https://github.com/$OWNER/$REPO/releases/latest/download/$asset"
    checksums_url="https://github.com/$OWNER/$REPO/releases/latest/download/checksums.txt"
  else
    archive_url="https://github.com/$OWNER/$REPO/releases/download/$VERSION/$asset"
    checksums_url="https://github.com/$OWNER/$REPO/releases/download/$VERSION/checksums.txt"
  fi
}

main() {
  os="$(detect_os)"
  arch="$(detect_arch)"
  asset="${BINARY}_${os}_${arch}.tar.gz"
  resolve_urls "$asset"

  if [ -z "$INSTALL_DIR" ]; then
    INSTALL_DIR="$(default_install_dir)"
  fi

  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' EXIT INT TERM

  archive="$tmpdir/$asset"
  checksums="$tmpdir/checksums.txt"

  download "$archive_url" "$archive"
  download "$checksums_url" "$checksums"

  expected="$(awk -v file="$asset" '$2 == file { print $1 }' "$checksums")"
  [ -n "$expected" ] || fail "checksum entry for $asset not found"

  actual="$(checksum_file "$archive")"
  [ "$expected" = "$actual" ] || fail "checksum verification failed for $asset"

  mkdir -p "$INSTALL_DIR"
  tar -xzf "$archive" -C "$tmpdir"

  if need_cmd install; then
    install "$tmpdir/$BINARY" "$INSTALL_DIR/$BINARY"
  else
    cp "$tmpdir/$BINARY" "$INSTALL_DIR/$BINARY"
    chmod 0755 "$INSTALL_DIR/$BINARY"
  fi

  printf '%s\n' "installed $BINARY to $INSTALL_DIR/$BINARY"
  case ":$PATH:" in
    *":$INSTALL_DIR:"*)
      ;;
    *)
      printf '%s\n' "note: $INSTALL_DIR is not on PATH" >&2
      ;;
  esac
}

main "$@"
