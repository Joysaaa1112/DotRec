# Third-party notices / 第三方组件说明

`DotRec.exe` embeds a statically linked `ffmpeg.exe` built from the sources below.
The exact build configuration is in [`scripts/build.sh`](scripts/build.sh), which
downloads these sources and reproduces the binary.

`DotRec.exe` 内嵌了一个静态链接的 `ffmpeg.exe`，由以下源码编译。完整编译参数见
[`scripts/build.sh`](scripts/build.sh)，运行该脚本即可从源码重新生成。

| Component | Version | License | Source |
|---|---|---|---|
| FFmpeg | `n7.1` | GPL-2.0-or-later (built with `--enable-gpl`) | https://github.com/FFmpeg/FFmpeg/tree/n7.1 |
| x264 | `c24e06c2e184345ceb33eb20a15d1024d9fd3497` | GPL-2.0-or-later | https://code.videolan.org/videolan/x264 (mirror: https://github.com/mirror/x264) |
| MinGW-w64 runtime (winpthreads) | from the build toolchain | MIT / ZPL | https://www.mingw-w64.org |

FFmpeg is a trademark of Fabrice Bellard, originator of the FFmpeg project.
DotRec is not affiliated with or endorsed by the FFmpeg project or VideoLAN.

The full GPL text is in [`LICENSE`](LICENSE).
