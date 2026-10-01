#!/bin/bash
# 全球鹰 GlobalHawk (Go + Wails) macOS arm64 构建脚本
set -e
cd "$(dirname "$0")"

echo "[1/4] 编译..."
CGO_LDFLAGS="-framework UniformTypeIdentifiers" \
  go build -tags desktop,production -trimpath -ldflags "-s -w" -o build/bin/GlobalHawk .

echo "[2/4] 组装 GlobalHawk.app ..."
APP=build/GlobalHawk.app
rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
cp build/bin/GlobalHawk "$APP/Contents/MacOS/GlobalHawk"
cp build/app.icns "$APP/Contents/Resources/app.icns" 2>/dev/null || true
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

echo "[3/4] 签名..."
codesign --force -s - "$APP"

echo "[4/4] 完成: $APP"
du -sh "$APP"
