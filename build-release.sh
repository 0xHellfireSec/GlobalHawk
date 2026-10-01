#!/bin/bash
# 全球鹰 GlobalHawk 多平台发布构建
# 产出：macOS arm64 / macOS x86_64 两个 DMG + Windows x64 zip（到 release/）
set -e
cd "$(dirname "$0")"

VERSION=1.0.0
OUT=release
LDFLAGS="-s -w"
rm -rf "$OUT"
mkdir -p "$OUT/darwin-arm64" "$OUT/darwin-x86_64" "$OUT/windows-x64"

assemble_app() { # $1=二进制  $2=目标.app
    APP="$2"
    rm -rf "$APP"
    mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
    cp "$1" "$APP/Contents/MacOS/GlobalHawk"
    cp build/app.icns "$APP/Contents/Resources/app.icns"
    cp PkgInfo "$APP/Contents/PkgInfo"
    cat > "$APP/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleName</key>              <string>GlobalHawk</string>
    <key>CFBundleDisplayName</key>       <string>全球鹰</string>
    <key>CFBundleIdentifier</key>        <string>com.hellfiresec.globalhawk</string>
    <key>CFBundleShortVersionString</key><string>1.0.0</string>
    <key>CFBundleVersion</key>           <string>1.0.0</string>
    <key>CFBundleExecutable</key>        <string>GlobalHawk</string>
    <key>CFBundleIconFile</key>          <string>app.icns</string>
    <key>CFBundlePackageType</key>       <string>APPL</string>
    <key>LSMinimumSystemVersion</key>    <string>11.0</string>
    <key>NSHighResolutionCapable</key>   <true/>
    <key>NSPrincipalClass</key>          <string>NSApplication</string>
</dict>
</plist>
PLIST
    codesign --force -s - "$APP"
}

make_dmg() { # $1=staging 目录  $2=输出dmg（含 Retina @2x 背景）
    local STAGE="$1" OUTDMG="$2"
    local TMPDMG="build/dmg-tmp.dmg"
    local MNT="/Volumes/GlobalHawk-tmp"
    # 模板已固化到仓库（源自 create-dmg, BSD）；本机若装了 create-dmg 优先用其模板
    local TEMPLATE="build/dmg-template.applescript"
    if [ -f "/opt/homebrew/Cellar/create-dmg/1.3.0/share/create-dmg/support/template.applescript" ]; then
        TEMPLATE="/opt/homebrew/Cellar/create-dmg/1.3.0/share/create-dmg/support/template.applescript"
    fi
    rm -f "$TMPDMG"
    hdiutil create -size 64m -fs HFS+J -volname "GlobalHawk-tmp" "$TMPDMG" >/dev/null
    hdiutil attach "$TMPDMG" -nobrowse -mountpoint "$MNT" >/dev/null

    ditto "$STAGE/GlobalHawk.app" "$MNT/GlobalHawk.app"
    ln -s /Applications "$MNT/Applications"
    mkdir -p "$MNT/.background"
    cp build/dmg-bg.png "$MNT/.background/dmg-bg.png"
    cp "build/dmg-bg@2x.png" "$MNT/.background/dmg-bg@2x.png"   # Retina 高清背景
    cp build/app.icns "$MNT/.VolumeIcon.icns"
    SetFile -c icnC "$MNT/.VolumeIcon.icns"

    # AppleScript 定位窗口/图标/背景（与 create-dmg 同款模板）
    local ASCRIPT
    ASCRIPT=$(mktemp -t globalhawk-dmg.XXXXXX)
    cat "$TEMPLATE" \
        | sed -e "s/WINX/200/g" -e "s/WINY/120/g" -e "s/WINW/660/g" -e "s/WINH/420/g" \
              -e "s|BACKGROUND_CLAUSE|set background picture of opts to file \".background:dmg-bg.png\"|g" \
              -e "s/REPOSITION_HIDDEN_FILES_CLAUSE//g" \
              -e "s/ICON_SIZE/96/g" -e "s/TEXT_SIZE/14/g" \
        | perl -pe 's:POSITION_CLAUSE:set position of item "GlobalHawk.app" to {160, 245}\n\t\t\tset position of item "Applications" to {500, 245}:g' \
        | perl -pe "s/QL_CLAUSE//g" \
        | perl -pe "s/APPLICATION_CLAUSE//g" \
        | perl -pe "s/HIDING_CLAUSE//" \
        > "$ASCRIPT"
    sleep 2
    /usr/bin/osascript "$ASCRIPT" "GlobalHawk-tmp" "$MNT" >/dev/null
    sleep 2
    hdiutil detach "$MNT" >/dev/null
    hdiutil convert "$TMPDMG" -format UDZO -imagekey zlib-level=9 -o "$OUTDMG" >/dev/null
    rm -f "$TMPDMG"
    hdiutil verify "$OUTDMG" >/dev/null
}

# ---------- macOS arm64 ----------
echo "[1/6] macOS arm64 ..."
CGO_LDFLAGS="-framework UniformTypeIdentifiers" \
  go build -tags desktop,production -trimpath -ldflags "$LDFLAGS" -o build/bin/GlobalHawk-arm64 .
assemble_app build/bin/GlobalHawk-arm64 "$OUT/darwin-arm64/GlobalHawk.app"
make_dmg "$OUT/darwin-arm64" "$OUT/GlobalHawk-${VERSION}-macOS-arm64.dmg"

# ---------- macOS x86_64 ----------
echo "[2/6] macOS x86_64 ..."
SDKROOT="$(xcrun --show-sdk-path)" CC="clang -arch x86_64" CGO_ENABLED=1 \
  GOOS=darwin GOARCH=amd64 \
  CGO_LDFLAGS="-framework UniformTypeIdentifiers" \
  go build -tags desktop,production -trimpath -ldflags "$LDFLAGS" -o build/bin/GlobalHawk-amd64 .
assemble_app build/bin/GlobalHawk-amd64 "$OUT/darwin-x86_64/GlobalHawk.app"
make_dmg "$OUT/darwin-x86_64" "$OUT/GlobalHawk-${VERSION}-macOS-x86_64.dmg"

# ---------- Windows x64 ----------
echo "[3/6] Windows x64 ..."
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go build -tags desktop,production -trimpath -ldflags "$LDFLAGS -H windowsgui" \
  -o "$OUT/windows-x64/GlobalHawk.exe" .
cp README.md "$OUT/windows-x64/使用说明.txt"

echo "[4/6] 密钥泄露扫描 ..."
LEAK=0
for KEY in $(python3 -c "
import configparser
cf = configparser.ConfigParser()
cf.read('$HOME/config.ini')
for sec in cf.sections():
    for k, v in cf[sec].items():
        if len(v.strip()) >= 16:
            print(v.strip())
"); do
    for F in $(find "$OUT" -type f \( -name "GlobalHawk" -o -name "GlobalHawk.exe" -o -name "*.plist" -o -name "*.txt" \)); do
        if grep -aq "$KEY" "$F"; then
            echo "!!! 泄露：$F 包含密钥 ${KEY:0:8}..."
            LEAK=1
        fi
    done
done
[ "$LEAK" -eq 0 ] && echo "扫描通过：所有产物不含任何 API Key" || { echo "存在泄露，终止发布"; exit 1; }

echo "[5/6] 打 Windows zip ..."
ditto -c -k "$OUT/windows-x64" "$OUT/GlobalHawk-${VERSION}-windows-x64.zip"

echo "[6/6] 完成。产物："
ls -lh "$OUT"/*.dmg "$OUT"/*.zip
