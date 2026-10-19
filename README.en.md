# DotRec 点录

English | [中文](README.md)

A tiny screen recorder for Windows. It lives in the system tray: right-click to start recording, and the MP4 it saves shows the mouse cursor.

- One exe, about 4 MB, no installer
- No window; everything is in the tray menu
- Records the mouse cursor (can be turned off)
- Pause and resume; the pieces are joined into one video automatically
- Global hotkeys: `Ctrl+Alt+R` start / stop, `Ctrl+Alt+P` pause / resume
- Records a microphone or any other audio input
- Chinese and English interface; settings are remembered

## Download

Get the latest `DotRec.exe` from [Releases](https://github.com/Joysaaa1112/DotRec/releases) and run it.

The exe is not code-signed, so Windows may show "Windows protected your PC" the first time. Click **More info** → **Run anyway**.

Requires Windows 10 or 11 (64-bit).

## Usage

After launch, the DotRec icon appears in the bottom-right of the taskbar (it may be hidden under `^`). Left- or right-click it to open the menu:

```
Start recording        Ctrl+Alt+R
Pause                  Ctrl+Alt+P
─────────────────────────────────
Area            ▸  Main screen / All screens
Frame rate      ▸  30 fps / 60 fps
Quality         ▸  Standard / High
Audio           ▸  No audio / microphones…
✓ Show cursor
─────────────────────────────────
Disable hotkeys
Open save folder
Play last recording
Language / 语言 ▸
─────────────────────────────────
Exit
```

- The icon turns red while recording and yellow when paused; hover over it to see the elapsed time
- Videos are saved to `Videos\DotRec`, named like `DotRec_2026-10-19_22-30-00.mp4`
- A notification appears when a video is saved; click it to show the file

### Recording system audio

DotRec records audio through DirectShow. To capture what your PC is playing, enable **Stereo Mix** under Windows Sound settings → Recording, then pick it from DotRec's **Audio** menu.

## How it works

The tray icon and menu are plain Win32 calls from Go. Recording is done by an embedded, stripped-down [FFmpeg](https://ffmpeg.org):

- `gdigrab` captures the screen (`-draw_mouse 1` draws the cursor)
- `dshow` captures audio
- `libx264` (`zerolatency`) encodes video, `aac` encodes audio
- Each pause closes an MKV segment; on stop, `concat` joins them losslessly into an MP4

Only those components are compiled in, so FFmpeg is about 4 MB instead of about 170 MB. It is unpacked to `%LOCALAPPDATA%\DotRec` on first run.

## Building from source

Cross-compile on Linux or WSL:

```bash
# Ubuntu / Debian
sudo apt install gcc-mingw-w64-x86-64 g++-mingw-w64-x86-64 nasm make pkg-config git golang
bash scripts/build.sh
# Output: dist/DotRec.exe
```

The script downloads and builds pinned versions of x264 and FFmpeg, then builds DotRec. The first build takes a few minutes.

Pushing a `v*` tag makes GitHub Actions build `DotRec.exe` and attach it to a release:

```bash
git tag v1.0.0
git push origin v1.0.0
```

## License

[GPL-3.0-or-later](LICENSE).

The embedded FFmpeg is built with x264, which is GPL-licensed, so the whole project is released under the GPL. See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) for third-party components.
