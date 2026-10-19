# 点录 DotRec

[English](README.en.md) | 中文

一个极简的 Windows 录屏工具：常驻任务栏托盘，右键开始录制，录出来的 MP4 里会显示鼠标指针。

- 单个 exe，约 4 MB，不用安装
- 没有窗口，所有操作都在托盘右键菜单里
- 录制时显示鼠标指针（可关闭）
- 支持暂停 / 继续，暂停前后的片段会自动合成一个视频
- 全局快捷键：`Ctrl+Alt+R` 开始 / 停止，`Ctrl+Alt+P` 暂停 / 继续
- 可录麦克风或其他音频输入设备
- 中文 / English 界面切换，设置自动保存

## 下载

到 [Releases](https://github.com/Joysaaa1112/DotRec/releases) 下载最新的 `DotRec.exe`，双击运行即可。

程序没有数字签名，第一次运行时 Windows 可能提示「Windows 已保护你的电脑」，点「更多信息」→「仍要运行」。

系统要求：Windows 10 / 11（64 位）。

## 使用

运行后任务栏右下角会出现点录图标（可能在 `^` 里）。左键或右键点击图标打开菜单：

```
开始录制              Ctrl+Alt+R
暂停                  Ctrl+Alt+P
─────────────────────────────────
录制范围        ▸  主屏幕 / 全部屏幕
帧率            ▸  30 fps / 60 fps
画质            ▸  标准 / 高清
声音            ▸  不录声音 / 麦克风…
✓ 显示鼠标指针
─────────────────────────────────
禁用快捷键
打开保存文件夹
播放最近的录屏
语言 / Language ▸
─────────────────────────────────
退出
```

- 录制中图标变红，暂停时变黄，鼠标悬停可看到已录时长
- 视频保存在「视频\DotRec」文件夹，文件名形如 `DotRec_2026-10-19_22-30-00.mp4`
- 保存完成会弹出通知，点击通知可直接定位到文件

### 录制电脑声音

点录通过 DirectShow 录音。想录电脑里播放的声音，需要在 Windows 声音设置 →「录制」中启用「立体声混音」（Stereo Mix），然后在点录的「声音」菜单里选择它。

## 工作原理

点录用 Go 直接调用 Win32 API 实现托盘和菜单，录制部分调用内嵌的精简版 [FFmpeg](https://ffmpeg.org)：

- `gdigrab` 截取屏幕（`-draw_mouse 1` 绘制鼠标指针）
- `dshow` 采集音频
- `libx264`（`zerolatency`）编码视频，`aac` 编码音频
- 每次暂停结束一段 MKV，停止时用 `concat` 无损合成 MP4

FFmpeg 只编译了上述用到的组件，体积约 4 MB（完整版约 170 MB）。首次运行时它会被解压到 `%LOCALAPPDATA%\DotRec`。

## 从源码构建

在 Linux 或 WSL 上交叉编译：

```bash
# Ubuntu / Debian
sudo apt install gcc-mingw-w64-x86-64 g++-mingw-w64-x86-64 nasm make pkg-config git golang
bash scripts/build.sh
# 输出：dist/DotRec.exe
```

脚本会下载并编译固定版本的 x264 与 FFmpeg，再编译点录。首次构建约需几分钟。

推送 `v*` 标签时，GitHub Actions 会自动构建并把 `DotRec.exe` 发布到 Releases：

```bash
git tag v1.0.0
git push origin v1.0.0
```

## 许可证

[GPL-3.0-or-later](LICENSE)。

点录内嵌的 FFmpeg 启用了 GPL 组件 x264，因此整个项目以 GPL 发布。第三方组件说明见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。
