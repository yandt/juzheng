#!/usr/bin/env bash
#
# 一键构建三平台产物并发布到 GitHub Release。
#
# 产物：
#   - macOS  : juzheng-v<ver>-macos-universal.dmg（arm64+amd64 通用，ad-hoc 签名）
#   - Windows: juzheng-v<ver>-windows-amd64.zip / -arm64.zip（各含对应架构 exe + helper）
#
# 版本号自动读自 internal/appinfo/appinfo.go。资产统一放 bin/release/。
#
# 用法：
#   scripts/release.sh                 # 构建全部并发布为 prerelease
#   scripts/release.sh --no-publish    # 只构建，不发布（本地校验用）
#   scripts/release.sh --release       # 发布为正式版（非 prerelease）
#   scripts/release.sh --force         # 目标 tag 已存在时删除重建
#   scripts/release.sh --skip-mac      # 跳过 macOS（如在非 mac 主机）
#   scripts/release.sh --skip-win      # 跳过 Windows
#
set -euo pipefail

cd "$(dirname "$0")/.."                 # 仓库根
export PATH="$HOME/go/bin:$PATH"        # task / wails3 装在这里

# ---------- 常量 ----------
APP=juzheng
RELEASE_DIR=bin/release
HELPER_EXE=internal/helperclient/helper-assets/juzheng-helper.exe
HELPER_TAGS="singbox,with_gvisor,with_quic,with_dhcp,with_wireguard,with_utls,with_acme,with_clash_api,with_tailscale,with_ccm,with_ocm,badlinkname,tfogo_checklinkname0"
HELPER_LDFLAGS="-checklinkname=0 -X internal/godebug.defaultGODEBUG=multipathtcp=0"
APP_LDFLAGS="-w -s -H windowsgui"

# ---------- 选项 ----------
PUBLISH=1; PRERELEASE=1; DO_MAC=1; DO_WIN=1; FORCE=0
while [ $# -gt 0 ]; do
  case "$1" in
    --no-publish) PUBLISH=0 ;;
    --release)    PRERELEASE=0 ;;
    --prerelease) PRERELEASE=1 ;;
    --skip-mac)   DO_MAC=0 ;;
    --skip-win)   DO_WIN=0 ;;
    --force)      FORCE=1 ;;
    -h|--help)    grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "未知参数: $1（-h 看帮助）" >&2; exit 1 ;;
  esac; shift
done

# ---------- 版本号 ----------
gover() { grep -E "Version$1[[:space:]]*=" internal/appinfo/appinfo.go | head -1 | sed -E 's/.*"([^"]+)".*/\1/'; }
VERSION="$(gover Major).$(gover Minor).$(gover Patch)"
TAG="v${VERSION}"
[ -n "$VERSION" ] && [ "$VERSION" != ".." ] || { echo "解析版本号失败" >&2; exit 1; }

ASSETS=()
log() { printf '\033[1;36m[release]\033[0m %s\n' "$*"; }

# ---------- 预检 ----------
need() { command -v "$1" >/dev/null 2>&1 || { echo "缺少依赖: $1" >&2; exit 1; }; }
need go; need git
[ "$PUBLISH" = 1 ] && { need gh; gh auth status >/dev/null 2>&1 || { echo "gh 未登录，先 gh auth login" >&2; exit 1; }; }
[ "$DO_WIN" = 1 ] && need zip
if [ "$DO_MAC" = 1 ]; then
  [ "$(uname)" = Darwin ] || { echo "macOS 产物需在 mac 主机构建（或加 --skip-mac）" >&2; exit 1; }
  need task; need hdiutil; need lipo
fi

log "版本 ${TAG}（publish=${PUBLISH} prerelease=${PRERELEASE} mac=${DO_MAC} win=${DO_WIN}）"
mkdir -p "$RELEASE_DIR"

# 若发布且 tag 已存在，提前判断（避免白构建）。
if [ "$PUBLISH" = 1 ] && gh release view "$TAG" >/dev/null 2>&1; then
  if [ "$FORCE" = 1 ]; then log "Release ${TAG} 已存在，--force 将删除重建。"
  else echo "Release ${TAG} 已存在。递增版本号，或加 --force 覆盖。" >&2; exit 1; fi
fi

# ---------- 前端 ----------
log "构建前端 dist…"
( cd frontend && npm run build >/dev/null 2>&1 )

# ---------- macOS ----------
build_mac() {
  log "构建 macOS universal .app…"
  task darwin:package:universal >/dev/null
  local dmg="$RELEASE_DIR/${APP}-v${VERSION}-macos-universal.dmg"
  rm -f "$dmg"
  local stage; stage="$(mktemp -d)"
  cp -R "bin/${APP}.app" "$stage/"
  ln -s /Applications "$stage/Applications"
  hdiutil create -volname "Juzheng ${VERSION}" -srcfolder "$stage" -ov -format UDZO "$dmg" >/dev/null
  rm -rf "$stage"
  ASSETS+=("$PWD/$dmg")
  log "→ $dmg ($(du -h "$dmg" | cut -f1))"
}

# ---------- Windows（单架构）----------
win_readme() {
  cat <<EOF
Juzheng for Windows ($1)

1. Run juzheng.exe. On first start the proxy core installs a helper
   service via UAC elevation.
2. For TUN (virtual NIC) mode you need wintun.dll:
   Download the $1 wintun.dll from https://www.wintun.net and place it at
   %ProgramData%\\Juzheng\\wintun.dll or in System32, then retry.
   (Not needed if you don't enable TUN mode.)
EOF
}

build_win() {
  local arch="$1"
  log "构建 Windows/${arch} helper.exe…"
  rm -f "$HELPER_EXE"
  GOOS=windows GOARCH="$arch" CGO_ENABLED=0 \
    go build -tags "$HELPER_TAGS" -ldflags "$HELPER_LDFLAGS" -o "$HELPER_EXE" ./helper/

  local syso="rsrc_windows_${arch}.syso"
  if [ ! -f "$syso" ]; then
    log "生成 ${syso}（图标+manifest）…"
    go run github.com/akavel/rsrc@latest -arch "$arch" \
      -ico build/windows/icon.ico -manifest build/windows/wails.exe.manifest -o "$syso"
  fi

  log "构建 Windows/${arch} app…"
  GOOS=windows GOARCH="$arch" CGO_ENABLED=0 \
    go build -tags production -trimpath -buildvcs=false -ldflags="$APP_LDFLAGS" -o "bin/${APP}-${arch}.exe" .

  local stage; stage="$(mktemp -d)"
  cp "bin/${APP}-${arch}.exe" "$stage/${APP}.exe"
  win_readme "$arch" > "$stage/README.txt"
  local zip="$PWD/$RELEASE_DIR/${APP}-v${VERSION}-windows-${arch}.zip"
  rm -f "$zip"; ( cd "$stage" && zip -q -r "$zip" . ); rm -rf "$stage"
  ASSETS+=("$zip")
  log "→ $zip ($(du -h "$zip" | cut -f1))"
}

# helper.exe 内嵌进 app，需与 app 架构一致；两架构轮流构建后还原为 amd64（仓库默认）。
restore_helper_amd64() {
  GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
    go build -tags "$HELPER_TAGS" -ldflags "$HELPER_LDFLAGS" -o "$HELPER_EXE" ./helper/ 2>/dev/null || true
}

# ---------- 执行 ----------
[ "$DO_MAC" = 1 ] && build_mac
if [ "$DO_WIN" = 1 ]; then
  build_win amd64
  build_win arm64
  log "还原 amd64 helper.exe…"; restore_helper_amd64
fi

log "产物清单："
for a in "${ASSETS[@]}"; do echo "  - $(basename "$a")"; done

# ---------- 发布 ----------
if [ "$PUBLISH" = 1 ]; then
  [ "$FORCE" = 1 ] && gh release view "$TAG" >/dev/null 2>&1 && \
    { log "删除旧 Release ${TAG}…"; gh release delete "$TAG" --yes --cleanup-tag 2>/dev/null || gh release delete "$TAG" --yes; }

  local_prev="$(git tag --list 'v*' --sort=-v:refname | grep -vx "$TAG" | head -1 || true)"
  notes="$(mktemp)"
  {
    echo "矩正 (Juzheng) ${TAG}"
    echo
    echo "## 变更"
    if [ -n "$local_prev" ]; then git log --no-merges --pretty='- %s' "${local_prev}..HEAD"; else echo "- 初始发布"; fi
    echo
    echo "## 下载"
    echo "- macOS（Apple Silicon + Intel 通用）：\`${APP}-v${VERSION}-macos-universal.dmg\`（未公证，首次打开需右键「打开」放行）"
    echo "- Windows x64：\`${APP}-v${VERSION}-windows-amd64.zip\`"
    echo "- Windows ARM64：\`${APP}-v${VERSION}-windows-arm64.zip\`（TUN 模式需自备对应架构 wintun.dll，见包内 README）"
  } > "$notes"

  flags=(--title "矩正 ${TAG}" --notes-file "$notes" --target main)
  [ "$PRERELEASE" = 1 ] && flags+=(--prerelease)
  log "创建 Release ${TAG}…"
  gh release create "$TAG" "${flags[@]}" "${ASSETS[@]}"
  rm -f "$notes"
  log "完成：$(gh release view "$TAG" --json url --jq .url)"
else
  log "已跳过发布（--no-publish）。产物在 ${RELEASE_DIR}/。"
fi
