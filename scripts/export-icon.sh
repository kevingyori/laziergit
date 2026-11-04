#!/bin/sh
# Export the approved macOS rendition for README and non-catalog fallbacks.
# Requires Xcode 26 or later, including Icon Composer.
set -eu
cd "$(dirname "$0")/.."
developer_dir=$(xcode-select -p)
ictool="$developer_dir/../Applications/Icon Composer.app/Contents/Executables/ictool"
if [ ! -x "$ictool" ]; then
    echo "Icon Composer's ictool is missing; select a full Xcode installation." >&2
    exit 1
fi
"$ictool" assets/AppIcon.icon --export-image \
    --output-file resources/icon.png --platform macOS --rendition Default \
    --width 1024 --height 1024 --scale 1
