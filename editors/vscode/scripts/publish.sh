#!/usr/bin/env bash
#
# Package ruxx-xml once per platform, bundling the matching ruxx binary so the
# extension works with no separate install. With VSCE_PAT and/or OVSX_PAT set
# it also publishes each VSIX to the VS Code Marketplace and Open VSX.
#
# Usage:
#   bash scripts/publish.sh                 # package, then publish if tokens are set
#   bash scripts/publish.sh --package-only  # package only (npm run package:targets)
#
set -euo pipefail

EXT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPO_ROOT="$(cd "$EXT_DIR/../.." && pwd)"

PACKAGE_ONLY=0
if [ "${1:-}" = "--package-only" ]; then
	PACKAGE_ONLY=1
fi

cd "$EXT_DIR"

VERSION="$(node -p "require('./package.json').version")"
COMMIT="$(git -C "$REPO_ROOT" rev-parse --short HEAD 2>/dev/null || echo unknown)"
DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
LDFLAGS="-X github.com/ppreeper/ruxx/internal/version.Version=$VERSION -X github.com/ppreeper/ruxx/internal/version.Commit=$COMMIT -X github.com/ppreeper/ruxx/internal/version.Date=$DATE"

TARGETS=(linux-x64 linux-arm64 darwin-x64 darwin-arm64 win32-x64 win32-arm64)

goos_goarch() {
	case "$1" in
	linux-x64) echo "linux amd64" ;;
	linux-arm64) echo "linux arm64" ;;
	darwin-x64) echo "darwin amd64" ;;
	darwin-arm64) echo "darwin arm64" ;;
	win32-x64) echo "windows amd64" ;;
	win32-arm64) echo "windows arm64" ;;
	*)
		echo "unknown target $1" >&2
		return 1
		;;
	esac
}

echo "ruxx-xml $VERSION ($COMMIT)"

# vsce runs the vscode:prepublish script (npm run build) itself, so dist/ is
# always rebuilt from the current source.

VSIXES=()

for target in "${TARGETS[@]}"; do
	read -r goos goarch <<<"$(goos_goarch "$target")"
	bin="ruxx"
	if [ "$goos" = "windows" ]; then
		bin="ruxx.exe"
	fi

	echo "==> $target (GOOS=$goos GOARCH=$goarch)"
	(
		cd "$REPO_ROOT"
		CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
			go build -trimpath -ldflags "$LDFLAGS" -o "$EXT_DIR/$bin" ./cmd/ruxx
	)
	if [ "$goos" != "windows" ]; then
		chmod +x "$EXT_DIR/$bin"
	fi

	vsix="$EXT_DIR/ruxx-xml-$VERSION@$target.vsix"
	npx --no-install vsce package --target "$target" --out "$vsix"

	# The binary is never committed; it only lives long enough to be packed.
	rm -f "$EXT_DIR/$bin"
	VSIXES+=("$vsix")
done

# A package without --target is the fallback VS Code uses on platforms that
# have no platform-specific package; it looks for ruxx on PATH instead.
echo "==> generic (binary resolved from PATH)"
generic="$EXT_DIR/ruxx-xml-$VERSION.vsix"
npx --no-install vsce package --out "$generic"
VSIXES+=("$generic")

echo
echo "Packaged:"
printf '  %s\n' "${VSIXES[@]}"

if [ "$PACKAGE_ONLY" = 1 ]; then
	exit 0
fi

if [ -z "${VSCE_PAT:-}" ] && [ -z "${OVSX_PAT:-}" ]; then
	cat <<EOF

Neither VSCE_PAT nor OVSX_PAT is set, so nothing was published.

Publish by hand with:
  npx vsce publish --packagePath ruxx-xml-$VERSION@*.vsix ruxx-xml-$VERSION.vsix -p <pat>
  npx ovsx publish ruxx-xml-$VERSION@<target>.vsix -p <open-vsx-token>
EOF
	exit 0
fi

# The Marketplace must receive every platform package (plus the universal
# fallback) for a version, otherwise VS Code on an unpublished platform reports
# the extension as "not available" for that architecture. Publish them all in a
# single vsce invocation, matching microsoft/vscode-platform-specific-sample;
# publishing one VSIX per invocation can leave targets missing from the listing.
if [ -n "${VSCE_PAT:-}" ]; then
	echo "==> vsce publish ${#VSIXES[@]} package(s)"
	npx --no-install vsce publish --packagePath "${VSIXES[@]}" -p "$VSCE_PAT"
fi

# Open VSX takes one VSIX per invocation.
if [ -n "${OVSX_PAT:-}" ]; then
	for vsix in "${VSIXES[@]}"; do
		echo "==> ovsx publish $(basename "$vsix")"
		npx --yes ovsx publish "$vsix" -p "$OVSX_PAT"
	done
fi
