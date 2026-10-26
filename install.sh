#!/bin/sh
# Installs the mediawiki command-line tool on macOS or Linux:
#
#   curl -fsSL https://raw.githubusercontent.com/neayi/Mediawiki-LLM-Cli/main/install.sh | sh
#
# Environment variables:
#   MEDIAWIKI_INSTALL_DIR  where to install (default: ~/.local/bin)
#   MEDIAWIKI_VERSION      version to install, e.g. 0.1.0 (default: latest)
set -eu

repo="neayi/Mediawiki-LLM-Cli"
install_dir="${MEDIAWIKI_INSTALL_DIR:-$HOME/.local/bin}"

if [ -n "${MEDIAWIKI_VERSION:-}" ]; then
	base_url="https://github.com/$repo/releases/download/v${MEDIAWIKI_VERSION#v}"
else
	base_url="https://github.com/$repo/releases/latest/download"
fi
base_url="${MEDIAWIKI_DOWNLOAD_URL:-$base_url}"

case "$(uname -s)" in
	Darwin) os=darwin ;;
	Linux) os=linux ;;
	*) echo "Unsupported system: $(uname -s). On Windows, use install.ps1." >&2; exit 1 ;;
esac
case "$(uname -m)" in
	x86_64 | amd64) arch=amd64 ;;
	arm64 | aarch64) arch=arm64 ;;
	*) echo "Unsupported processor: $(uname -m)" >&2; exit 1 ;;
esac

url="$base_url/mediawiki-$os-$arch"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

echo "Downloading $url"
if command -v curl >/dev/null 2>&1; then
	curl -fSL --progress-bar -o "$tmp" "$url"
elif command -v wget >/dev/null 2>&1; then
	wget -q -O "$tmp" "$url"
else
	echo "curl or wget is required." >&2
	exit 1
fi

mkdir -p "$install_dir"
chmod +x "$tmp"
mv "$tmp" "$install_dir/mediawiki"
# Files downloaded by a browser are blocked by Gatekeeper; curl does not set
# the quarantine flag, but remove it in case.
if [ "$os" = darwin ]; then
	xattr -d com.apple.quarantine "$install_dir/mediawiki" 2>/dev/null || true
fi
echo "Installed $("$install_dir/mediawiki" version) in $install_dir"

# Add the install directory to the PATH of future shells.
case ":$PATH:" in
	*":$install_dir:"*) ;;
	*)
		case "${SHELL:-}" in
			*/zsh) rc="$HOME/.zshrc" ;;
			*/bash) if [ "$os" = darwin ]; then rc="$HOME/.bash_profile"; else rc="$HOME/.bashrc"; fi ;;
			*) rc="$HOME/.profile" ;;
		esac
		line="export PATH=\"$install_dir:\$PATH\""
		if ! grep -qsF "$line" "$rc"; then
			printf '\n# Added by the mediawiki installer\n%s\n' "$line" >>"$rc"
			echo "Added $install_dir to the PATH in $rc"
		fi
		echo "Open a new terminal, then run: mediawiki help"
		exit 0
		;;
esac
echo "Run: mediawiki help"
