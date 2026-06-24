#!/bin/sh
set -eu

REPO="tttpeng/grove"
BIN="grove"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$arch" in
  x86_64 | amd64) arch="amd64" ;;
  arm64 | aarch64) arch="arm64" ;;
  *) echo "grove: unsupported architecture: $arch" >&2; exit 1 ;;
esac
case "$os" in
  darwin | linux) ;;
  *) echo "grove: unsupported OS: $os" >&2; exit 1 ;;
esac

tag=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' | head -1 | cut -d '"' -f 4)
if [ -z "${tag}" ]; then
  echo "grove: could not determine latest release" >&2
  exit 1
fi

url="https://github.com/${REPO}/releases/download/${tag}/${BIN}_${os}_${arch}.tar.gz"
dest="${GROVE_INSTALL_DIR:-${HOME}/.local/bin}"

tmp=$(mktemp -d)
trap 'rm -rf "${tmp}"' EXIT

echo "grove: downloading ${tag} (${os}/${arch})…"
curl -fsSL "${url}" | tar -xz -C "${tmp}"
mkdir -p "${dest}"
mv "${tmp}/${BIN}" "${dest}/${BIN}"
chmod +x "${dest}/${BIN}"

echo "grove: installed to ${dest}/${BIN}"
case ":${PATH}:" in
  *":${dest}:"*) ;;
  *) echo "grove: add ${dest} to your PATH to run 'grove'" ;;
esac
