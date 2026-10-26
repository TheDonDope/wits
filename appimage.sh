#!/usr/bin/env bash
#
# Builds wits as an AppImage: the server, the command line, and the interface
# in a window of its own — Electron, carrying its Chromium — in one file that
# runs on any x86-64 Linux.
#
#   ./appimage.sh            dist/wits-<version>-x86_64.AppImage, and its SHA256SUMS
#   VERSION=v1.2.0 ./appimage.sh
#   ./appimage.sh check      run the image built, in a throwaway home: a first
#                            launch makes its repository and serves the
#                            interface, a second finds it running, the window
#                            loads it (given a display, or xvfb-run), cli answers
#
# Needs Go, Node with npm, and the network once, for appimagetool and the
# AppImage runtime — both pinned to a release and checked against its SHA-256,
# kept under $WITS_TOOLS (default ~/.cache/wits-dev) — and for Electron, pinned
# in desktop/package-lock.json. NSS, NSPR and ALSA are taken from the build
# machine and carried: Chromium needs them, and some systems (SteamOS) lack them.

set -euo pipefail
cd "$(dirname "$0")"

version="${VERSION:-$(git describe --tags --always --dirty)}"
arch=x86_64
goarch=amd64
tools="${WITS_TOOLS:-$HOME/.cache/wits-dev}/appimage"
appdir=build/AppDir
out=dist

# appimagetool, and the runtime it puts in front of the image — pinned, so a
# build does not take whatever "continuous" is that day.
tool_url="https://github.com/AppImage/appimagetool/releases/download/1.9.1/appimagetool-x86_64.AppImage"
tool_sha=ed4ce84f0d9caff66f50bcca6ff6f35aae54ce8135408b3fa33abfc3cb384eb0
runtime_url="https://github.com/AppImage/type2-runtime/releases/download/20251108/runtime-x86_64"
runtime_sha=2fca8b443c92510f1483a883f60061ad09b46b978b2631c807cd873a47ec260d

step() { printf '· %s\n' "$*"; }

# check runs the image the way a person would, in a home of its own.
check() {
	# home and launched outlive the function: the trap that clears them runs at exit.
	local image="$out/wits-$version-$arch.AppImage" addr=127.0.0.1:7399
	[ -x "$image" ] || { echo "✗ no $image: build it first" >&2; exit 1; }
	home="$(mktemp -d)"
	# The runtime passes no signal on to what it runs, so the image runs in a
	# session of its own, and the whole of it is stopped: launcher and server.
	trap 'kill -- -"$launched" 2>/dev/null; rm -rf "$home"' EXIT
	export APPIMAGE_EXTRACT_AND_RUN=1 XDG_DATA_HOME="$home" WITS_NO_BROWSER=1 WITS_ADDR="$addr"
	setsid "$image" >"$home/log" 2>&1 &
	launched=$!
	for _ in $(seq 1 100); do
		curl -fs -o /dev/null "http://$addr/api/overview" && break
		sleep 0.2
	done
	curl -fs -o /dev/null "http://$addr/api/overview" || { cat "$home/log"; echo "✗ the server did not answer" >&2; exit 1; }
	[ -d "$home/wits/.wits" ] || { echo "✗ no repository made in $home/wits" >&2; exit 1; }
	curl -fs "http://$addr/" | grep -q '<title>wits</title>' || { echo "✗ the interface is not served" >&2; exit 1; }
	"$image" | grep -q 'already running' || { echo "✗ a second launch did not find the first" >&2; exit 1; }
	window "$image"
	"$image" cli --version | grep -q "$version" || { echo "✗ cli does not report $version" >&2; exit 1; }
	step "checked: $image serves, makes its repository, knows it runs, opens its window on it, and answers as cli"
}

# window opens the image's window on the server running, and checks it loads
# the interface: on the display there is, or on one xvfb-run makes. Chromium
# without any display is not tried — headless, it crashes before it loads.
window() {
	local run=()
	if [ -z "${DISPLAY:-}${WAYLAND_DISPLAY:-}" ]; then
		command -v xvfb-run >/dev/null || { step "window not checked: no display, and no xvfb-run"; return; }
		run=(xvfb-run -a)
	fi
	env -u WITS_NO_BROWSER WITS_SMOKE=1 "${run[@]}" "$1" 2>&1 | grep -q 'wits-desktop: loaded wits' ||
		{ echo "✗ the window did not load the interface" >&2; exit 1; }
}

# carried is what Chromium links against that a system may not have: NSS (and
# the modules it loads itself), NSPR, and ALSA.
carried=(libnspr4.so libplc4.so libplds4.so libnss3.so libnssutil3.so libsmime3.so libssl3.so
	libsoftokn3.so libfreebl3.so libfreeblpriv3.so libasound.so.2)

# library finds one on the build machine: where LD_LIBRARY_PATH says first
# (./doctor.sh env, on a Steam Deck), then the system's own folders — Debian's
# keep NSS's modules in an nss/ of their own.
library() {
	local dirs=() dir
	IFS=: read -ra dirs <<<"${LD_LIBRARY_PATH:-}"
	for dir in "${dirs[@]}" /usr/lib/x86_64-linux-gnu /usr/lib/x86_64-linux-gnu/nss /usr/lib64 /usr/lib; do
		[ -n "$dir" ] && [ -e "$dir/$1" ] && { echo "$dir/$1"; return; }
	done
	echo "✗ $1 is not on this machine, and the image carries it: install NSS and ALSA (or ./doctor.sh install)" >&2
	exit 1
}

# fetch downloads a file once, and refuses one that is not what was pinned.
fetch() {
	local url="$1" sha="$2" dest="$3"
	if [ -f "$dest" ] && echo "$sha  $dest" | sha256sum -c --status; then
		return
	fi
	step "fetching $(basename "$url")"
	mkdir -p "$(dirname "$dest")"
	curl -fsSL -o "$dest.part" "$url"
	if ! echo "$sha  $dest.part" | sha256sum -c --status; then
		rm -f "$dest.part"
		echo "✗ $(basename "$url") is not the file pinned (sha256 $sha)" >&2
		exit 1
	fi
	mv "$dest.part" "$dest"
	chmod +x "$dest"
}

if [ "${1:-}" = check ]; then
	check
	exit 0
fi

fetch "$tool_url" "$tool_sha" "$tools/appimagetool-1.9.1-x86_64.AppImage"
fetch "$runtime_url" "$runtime_sha" "$tools/runtime-20251108-x86_64"

pkg=github.com/TheDonDope/wits/pkg/version
ldflags="-s -w -X $pkg.Version=$version -X $pkg.CommitSHA=$(git rev-parse --short HEAD)"
ldflags+=" -X $pkg.CommitDate=$(git --no-pager log -1 --pretty='format:%cd' --date='format:%Y-%m-%dT%H:%M:%S')"

rm -rf "$appdir"
mkdir -p "$appdir/usr/bin" "$appdir/usr/lib" "$appdir/usr/share/wits" "$appdir/usr/share/applications" \
	"$appdir/usr/share/icons/hicolor/512x512/apps" "$out"

# Static binaries: nothing from the system they land on but its kernel.
for cmd in wits wits-server; do
	step "building $cmd $version"
	CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" go build -trimpath -ldflags "$ldflags" -o "$appdir/usr/bin/$cmd" "./cmd/$cmd"
done

step "building the interface"
(
	cd wits-ui
	[ -d node_modules ] || npm ci
	npx ng build --define "WITS_VERSION='$version'" >/dev/null
)
# -L: the demo bundle is a link to assets/demo-seed.wits, and must travel as the file.
cp -rL wits-ui/dist/wits-ui/browser "$appdir/usr/share/wits/ui"
# The phone's engine, if a `make wasm` left one in public/: the server's
# interface never loads it, so it is 13 MB the AppImage need not carry.
rm -f "$appdir/usr/share/wits/ui/wits.wasm" "$appdir/usr/share/wits/ui/wasm_exec.js"

# The window: Electron's own build, with wits' shell as its app. Its default
# app and every locale but English are left out — the interface is English.
(
	cd desktop
	npm ci --no-audit --no-fund >/dev/null
	# npm runs no install script unasked, and Electron's fetches its build.
	node node_modules/electron/install.js
)
step "adding the window: Electron $(node -p 'require("./desktop/node_modules/electron/package.json").version')"
shell="$appdir/usr/lib/wits-desktop"
cp -r desktop/node_modules/electron/dist "$shell"
mv "$shell/electron" "$shell/wits-desktop"
rm -f "$shell/resources/default_app.asar"
find "$shell/locales" -name '*.pak' ! -name 'en-US.pak' -delete
mkdir -p "$shell/resources/app" "$shell/lib"
cp desktop/main.js desktop/package.json "$shell/resources/app/"
for lib in "${carried[@]}"; do
	cp -L "$(library "$lib")" "$shell/lib/$lib"
done
# Debian's NSS looks for its modules in nss/ beside it; this one, in its own folder.
ln -s . "$shell/lib/nss"

cp packaging/appimage/AppRun "$appdir/AppRun"
chmod +x "$appdir/AppRun"
cp packaging/appimage/wits.desktop "$appdir/wits.desktop"
cp packaging/appimage/wits.desktop "$appdir/usr/share/applications/wits.desktop"
cp wits-ui/public/icons/icon-512.png "$appdir/wits.png"
cp wits-ui/public/icons/icon-512.png "$appdir/usr/share/icons/hicolor/512x512/apps/wits.png"
ln -sf wits.png "$appdir/.DirIcon"

image="$out/wits-$version-$arch.AppImage"
step "packing $image"
# Extracted rather than mounted: a build machine (or a CI runner) may have no FUSE.
APPIMAGE_EXTRACT_AND_RUN=1 ARCH="$arch" "$tools/appimagetool-1.9.1-x86_64.AppImage" \
	--no-appstream --runtime-file "$tools/runtime-20251108-x86_64" "$appdir" "$image" >/dev/null
(cd "$out" && sha256sum "$(basename "$image")" >SHA256SUMS)
step "done: $image ($(du -h "$image" | cut -f1))"
