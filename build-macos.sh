#!/bin/bash
# Optional SOURCE build only. The distributed .app does not need build tools.
# Requires macOS, Xcode Command Line Tools, Go, and Python 3 for plist/icon copying.
# Apple-provided codesign produces an ad-hoc signature here, not Developer ID.
set -euo pipefail
cd "$(dirname "$0")"
command -v go >/dev/null
xcrun --find clang >/dev/null
APP="$PWD/mac-build/M175 Studio.app"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
cp native/Info.plist "$APP/Contents/Info.plist"
cp native/AppIcon.icns "$APP/Contents/Resources/AppIcon.icns"
cp GO-LICENSE.txt "$APP/Contents/Resources/GO-LICENSE.txt"
cp NOTICE.txt "$APP/Contents/Resources/NOTICE.txt"
printf 'APPL????' > "$APP/Contents/PkgInfo"
(cd engine && go test ./...)
for pair in 'arm64 arm64' 'amd64 x86_64'; do
  read -r goarch macarch <<< "$pair"
  (cd engine && GOOS=darwin GOARCH="$goarch" CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "$APP/Contents/Resources/m175-engine-$macarch" .)
  # Go 1.23 internal linking does not emit LC_UUID. Add one to this self-built
  # executable, then let macOS codesign replace the intermediate ad-hoc signature.
  python3 sign_macho.py "$APP/Contents/Resources/m175-engine-$macarch" "local.m175.studio.engine.$macarch"
  /usr/bin/codesign --force --sign - --timestamp=none "$APP/Contents/Resources/m175-engine-$macarch"
done
xcrun clang -arch arm64 -arch x86_64 -mmacosx-version-min=12.0 -O2 -fblocks -fno-objc-arc -framework Cocoa -framework WebKit native/Studio.m -o "$APP/Contents/MacOS/M175Studio"
/usr/bin/codesign --force --sign - --timestamp=none "$APP"
/usr/bin/codesign --verify --deep --strict --verbose=2 "$APP"
printf '\nBuilt: %s\nThis is ad-hoc signed; it is not Apple-notarized.\n' "$APP"
