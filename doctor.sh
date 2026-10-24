#!/usr/bin/env bash
#
# Everything the development loop leans on, checked in one place — and
# installed, user-locally, when asked.
#
# Building and testing need only Go. The rest is the loop around it: the
# release (gh, git-chglog), the gate (shellcheck, lizard), and the tapes (vhs,
# gum, ttyd, ffmpeg, the Hack font, and a Chromium that can start). Each was
# found missing at the moment it was needed; this finds them first.
#
#   ./doctor.sh            say what is there and what is missing
#   ./doctor.sh install    fetch what is missing — no sudo, nothing system-wide
#   eval "$(./doctor.sh env)"
#                          export what vhs needs to start its Chromium
#
# Binaries land in ~/.local/bin and $GOPATH/bin. lizard and openpyxl get a venv and
# the Chromium libraries their own directory, both under $WITS_TOOLS
# (default ~/.cache/wits-dev), which preflight.sh and render-vhs-tapes.sh read.

set -euo pipefail

tools="${WITS_TOOLS:-$HOME/.cache/wits-dev}"
bin="$HOME/.local/bin"
gobin="$(go env GOBIN 2>/dev/null || true)"
[ -n "$gobin" ] || gobin="$(go env GOPATH 2>/dev/null || echo "$HOME/go")/bin"
missing=0

say() { printf '%s\n' "$*"; }
ok() { say "✓ $1"; }
no() {
	say "✗ $1 — $2"
	missing=1
}
has() { command -v "$1" >/dev/null 2>&1 || [ -x "$gobin/$1" ] || [ -x "$bin/$1" ]; }

# Lists are read whole before grepping: `cmd | grep -q` lets grep exit on the
# first match, the writer dies of SIGPIPE, and pipefail calls the find a miss.
has_font() { [[ "$(fc-list 2>/dev/null)" == *Hack* ]]; }

# venv_ready says whether the Python tools are in their venv: lizard for the
# gate's complexity check, openpyxl for reading the workbook's cells without
# going through the importer — the independent check its tests are pinned by.
venv_ready() { [ -x "$tools/venv/bin/python" ] && "$tools/venv/bin/python" -c 'import lizard, openpyxl' 2>/dev/null; }

# chromium_libs lists the shared libraries a vhs Chromium needs and this
# system does not ship. SteamOS lacks all three.
chromium_libs() {
	known="$(ldconfig -p 2>/dev/null || true)"
	for lib in libnss3.so libnspr4.so libasound.so.2; do
		if [[ "$known" != *"$lib"* ]] && [ ! -e "$tools/chromelibs/usr/lib/$lib" ]; then
			echo "$lib"
		fi
	done
}

check() {
	if has go; then ok "go $(go env GOVERSION)"; else no go "install Go from go.dev; nothing builds without it"; fi
	if has gh && gh auth status >/dev/null 2>&1; then
		ok "gh, signed in"
	elif has gh; then
		no gh "installed but signed out: gh auth login"
	else
		no gh "needed for pull requests and releases"
	fi
	for t in git-chglog vhs gum ttyd ffmpeg shellcheck; do
		if has "$t"; then ok "$t"; else no "$t" "./doctor.sh install"; fi
	done
	if venv_ready; then
		ok "lizard and openpyxl (in $tools/venv)"
	else
		no "lizard and openpyxl" "./doctor.sh install; preflight skips the complexity check without lizard"
	fi
	if has_font; then ok "the Hack font"; else no "the Hack font" "./doctor.sh install"; fi
	libs="$(chromium_libs)"
	if [ -z "$libs" ]; then
		ok "Chromium libraries for vhs"
	else
		no "Chromium libraries for vhs" "missing $(echo "$libs" | tr '\n' ' ')— ./doctor.sh install"
	fi
	if [ -d "$bin" ] && [[ ":$PATH:" != *":$bin:"* ]]; then say "! $bin is not on PATH"; fi
	if [[ ":$PATH:" != *":$gobin:"* ]]; then say "! $gobin is not on PATH"; fi
	say ""
	if [ "$missing" -eq 0 ]; then say "all set"; else say "./doctor.sh install fetches what is missing"; fi
	return "$missing"
}

# fetch downloads a URL into the scratch directory, failing loudly.
fetch() { curl -fsSL -o "$2" "$1"; }

install_tools() {
	mkdir -p "$bin" "$tools"
	scratch="$(mktemp -d)"
	trap 'rm -rf "$scratch"' EXIT

	has git-chglog || go install github.com/git-chglog/git-chglog/cmd/git-chglog@latest
	has vhs || go install github.com/charmbracelet/vhs@latest
	has gum || go install github.com/charmbracelet/gum@latest

	if ! has ttyd; then
		fetch https://github.com/tsl0922/ttyd/releases/download/1.7.7/ttyd.x86_64 "$scratch/ttyd"
		install -m755 "$scratch/ttyd" "$bin/ttyd"
	fi
	if ! has ffmpeg; then
		fetch https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-amd64-static.tar.xz "$scratch/ff.tar.xz"
		tar -xJf "$scratch/ff.tar.xz" -C "$scratch"
		install -m755 "$scratch"/ffmpeg-*-static/ffmpeg "$scratch"/ffmpeg-*-static/ffprobe "$bin/"
	fi
	if ! has shellcheck; then
		fetch https://github.com/koalaman/shellcheck/releases/download/v0.11.0/shellcheck-v0.11.0.linux.x86_64.tar.xz "$scratch/sc.tar.xz"
		tar -xJf "$scratch/sc.tar.xz" -C "$scratch"
		install -m755 "$scratch/shellcheck-v0.11.0/shellcheck" "$bin/shellcheck"
	fi
	if ! venv_ready; then
		python3 -m venv "$tools/venv"
		"$tools/venv/bin/pip" install -q lizard openpyxl
	fi
	if ! has_font; then
		fetch https://github.com/source-foundry/Hack/releases/download/v3.003/Hack-v3.003-ttf.zip "$scratch/hack.zip"
		mkdir -p "$HOME/.local/share/fonts"
		(cd "$scratch" && unzip -qo hack.zip && cp ttf/*.ttf "$HOME/.local/share/fonts/")
		fc-cache -f >/dev/null 2>&1 || true
	fi
	if [ -n "$(chromium_libs)" ]; then
		# Arch's own packages, unpacked rather than installed: the libraries
		# land in one directory that only vhs is pointed at, via `env`.
		mirror=https://geo.mirror.pkgbuild.com
		for spec in core/nss core/nspr extra/alsa-lib; do
			repo="${spec%/*}"
			pkg="${spec#*/}"
			file="$(curl -fsSL "$mirror/$repo/os/x86_64/" |
				grep -oE "\"$pkg-[0-9][^\"]*-x86_64\.pkg\.tar\.zst\"" | tr -d '"' | sort -V | tail -1)"
			fetch "$mirror/$repo/os/x86_64/$file" "$scratch/$file"
			mkdir -p "$tools/chromelibs"
			tar --zstd -xf "$scratch/$file" -C "$tools/chromelibs" usr/lib
		done
	fi
	missing=0
	say ""
	check
}

# env prints the exports vhs needs. Empty when the system has the libraries.
env_exports() {
	if [ -d "$tools/chromelibs/usr/lib" ]; then
		echo "export LD_LIBRARY_PATH=\"$tools/chromelibs/usr/lib\${LD_LIBRARY_PATH:+:\$LD_LIBRARY_PATH}\""
	fi
}

case "${1:-check}" in
check) check ;;
install) install_tools ;;
env) env_exports ;;
*)
	say "usage: ./doctor.sh [check | install | env]"
	exit 2
	;;
esac
