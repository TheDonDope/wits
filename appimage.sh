#!/usr/bin/env bash
#
# Builds wits as an AppImage: the server, the command line, and the browser
# interface bundled beside them, in one file that runs on any x86-64 Linux.
#
#   ./appimage.sh            dist/wits-<version>-x86_64.AppImage, and its SHA256SUMS
#   VERSION=v1.2.0 ./appimage.sh
#   ./appimage.sh check      run the image built, in a throwaway home: a first
#                            launch makes its repository and serves the
#                            interface, a second finds it running, cli answers
#
# Needs Go, Node with npm, and the network once, for appimagetool and the
# AppImage runtime: both pinned to a release and checked against its SHA-256,
# kept under $WITS_TOOLS (default ~/.cache/wits-dev).

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
	"$image" cli --version | grep -q "$version" || { echo "✗ cli does not report $version" >&2; exit 1; }
	step "checked: $image serves, makes its repository, knows it runs, and answers as cli"
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
mkdir -p "$appdir/usr/bin" "$appdir/usr/share/wits" "$appdir/usr/share/applications" \
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
