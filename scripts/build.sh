#!/usr/bin/env bash
# 从源码构建 点录 DotRec（Windows x64）。
# 在 Linux 或 WSL 上交叉编译，依赖：
#   gcc-mingw-w64-x86-64  g++-mingw-w64-x86-64  nasm  make  pkg-config  git  go (>= 1.24)
# Ubuntu / Debian:
#   sudo apt install gcc-mingw-w64-x86-64 g++-mingw-w64-x86-64 nasm make pkg-config git golang
#
# 用法：scripts/build.sh            生成 dist/DotRec.exe
set -euo pipefail

FFMPEG_TAG="n7.1"
X264_COMMIT="c24e06c2e184345ceb33eb20a15d1024d9fd3497"
HOST="x86_64-w64-mingw32"

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BUILD="$ROOT/build"
PREFIX="$BUILD/prefix"
JOBS="$(nproc 2>/dev/null || echo 4)"
mkdir -p "$BUILD" "$ROOT/dist"

# ---------- x264 ----------
if [ ! -f "$PREFIX/lib/libx264.a" ]; then
  echo "==> 编译 x264 ($X264_COMMIT)"
  rm -rf "$BUILD/x264"
  git init -q "$BUILD/x264"
  git -C "$BUILD/x264" fetch -q --depth 1 https://github.com/mirror/x264.git "$X264_COMMIT"
  git -C "$BUILD/x264" checkout -q FETCH_HEAD
  (cd "$BUILD/x264" && ./configure --host="$HOST" --cross-prefix="$HOST-" --prefix="$PREFIX" \
      --enable-static --disable-cli --disable-opencl --disable-avs --disable-swscale \
      --disable-lavf --disable-ffms --disable-gpac --disable-lsmash \
      --bit-depth=8 --chroma-format=420 \
    && make -j"$JOBS" && make install)
fi

# ---------- FFmpeg（只保留点录需要的部分） ----------
if [ ! -f "$BUILD/ffmpeg/ffmpeg.exe" ]; then
  echo "==> 编译 FFmpeg ($FFMPEG_TAG)"
  rm -rf "$BUILD/ffmpeg"
  git clone -q --depth 1 --branch "$FFMPEG_TAG" https://github.com/FFmpeg/FFmpeg.git "$BUILD/ffmpeg"
  (cd "$BUILD/ffmpeg" && PKG_CONFIG_PATH="$PREFIX/lib/pkgconfig" ./configure \
      --arch=x86_64 --target-os=mingw32 --cross-prefix="$HOST-" --pkg-config=pkg-config \
      --prefix="$PREFIX" --enable-static --disable-shared --enable-gpl --enable-libx264 \
      --extra-ldflags="-static -static-libgcc" --pkg-config-flags=--static \
      --disable-everything --disable-autodetect --disable-doc --disable-debug --disable-network \
      --disable-ffprobe --disable-ffplay --enable-small \
      --enable-indev=gdigrab,dshow \
      --enable-encoder=libx264,aac \
      --enable-decoder=bmp,pcm_s16le,h264,aac,rawvideo \
      --enable-muxer=mp4,matroska --enable-demuxer=concat,matroska \
      --enable-parser=h264,aac \
      --enable-bsf=aac_adtstoasc,h264_mp4toannexb,extract_extradata \
      --enable-protocol=file,pipe \
      --enable-filter=scale,format,aresample,null,anull,copy,acopy,aformat \
      --enable-swscale --enable-swresample \
    && make -j"$JOBS" && "$HOST-strip" -s ffmpeg.exe)
fi

# ---------- 嵌入 FFmpeg ----------
echo "==> 打包 FFmpeg"
gzip -9 -n -c "$BUILD/ffmpeg/ffmpeg.exe" > "$ROOT/ffmpeg.exe.gz"
FF_SIZE="$(wc -c < "$BUILD/ffmpeg/ffmpeg.exe" | tr -d ' ')"
FF_VER="$(sha256sum "$ROOT/ffmpeg.exe.gz" | cut -c1-12)"
cat > "$ROOT/ffmeta.go" <<EOF
// 由 scripts/build.sh 生成，请勿手动修改。
package main

const (
	ffVersion = "$FF_VER"
	ffSize    = $FF_SIZE
)
EOF

# ---------- 图标与清单 ----------
echo "==> 生成 Windows 资源"
(cd "$ROOT" && "$HOST-windres" -O coff -i app.rc -o rsrc_windows_amd64.syso)

# ---------- 点录 ----------
echo "==> 编译 DotRec.exe"
(cd "$ROOT" && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags "-H windowsgui -s -w" -o dist/DotRec.exe .)

ls -l "$ROOT/dist/DotRec.exe"
echo "==> 完成：dist/DotRec.exe"
