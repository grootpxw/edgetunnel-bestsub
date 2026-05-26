#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RELEASE_DIR="$ROOT_DIR/release"
VERSION="${1:-}"
UPLOAD_FLAG="${2:-}"
ASSETS=()

if [[ -z "$VERSION" ]]; then
  echo "用法: ./scripts/release-local.sh <tag> [--upload]"
  echo "示例: ./scripts/release-local.sh v1.0.1"
  echo "示例: ./scripts/release-local.sh v1.0.1 --upload"
  exit 1
fi

if ! command -v wails >/dev/null 2>&1; then
  echo "未检测到 wails，请先安装："
  echo "go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0"
  exit 1
fi

mkdir -p "$RELEASE_DIR"

GOOS_NAME="$(go env GOOS)"
GOARCH_NAME="$(go env GOARCH)"

build_macos() {
  local asset_name="$1"
  local platform="$2"
  echo "开始构建 $platform ..."
  wails build -clean -platform "$platform"
  rm -f "$RELEASE_DIR/$asset_name"
  ditto -c -k --sequesterRsrc --keepParent "$ROOT_DIR/build/bin/BestSub.app" "$RELEASE_DIR/$asset_name"
  ASSETS+=("$RELEASE_DIR/$asset_name")
}

build_windows() {
  local asset_name="BestSub-windows-amd64.zip"
  echo "开始构建 windows/amd64 ..."
  wails build -clean -platform windows/amd64 -webview2 download
  rm -f "$RELEASE_DIR/$asset_name"
  ditto -c -k "$ROOT_DIR/build/bin/BestSub.exe" "$RELEASE_DIR/$asset_name"
  ASSETS+=("$RELEASE_DIR/$asset_name")
}

case "$GOOS_NAME/$GOARCH_NAME" in
  darwin/arm64|darwin/amd64)
    build_macos "BestSub-darwin-arm64.zip" "darwin/arm64"
    build_windows
    ;;
  windows/amd64)
    echo "当前脚本请在 PowerShell 下运行 scripts/release-local.ps1"
    exit 1
    ;;
  *)
    echo "当前平台 $GOOS_NAME/$GOARCH_NAME 暂未内置完整本地 release 打包流程。"
    echo "推荐在 macOS 上执行本脚本，一次生成 macOS 与 Windows 包。"
    exit 1
    ;;
esac

printf '构建完成：\n'
for asset in "${ASSETS[@]}"; do
  echo "- $asset"
done

if [[ "$UPLOAD_FLAG" != "--upload" ]]; then
  echo "如需上传到 GitHub Release，请追加 --upload"
  exit 0
fi

if ! command -v gh >/dev/null 2>&1; then
  echo "未检测到 gh，请先安装 GitHub CLI 后重试上传。"
  exit 1
fi

if ! gh release view "$VERSION" >/dev/null 2>&1; then
  echo "Release $VERSION 不存在，正在创建..."
  gh release create "$VERSION" "${ASSETS[@]}" --title "BestSub $VERSION" --generate-notes
else
  echo "Release $VERSION 已存在，正在上传/覆盖资产..."
  gh release upload "$VERSION" "${ASSETS[@]}" --clobber
fi

echo "上传完成：$VERSION"
