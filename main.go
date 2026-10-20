//go:build windows

// 点录 DotRec —— 托盘录屏工具，录制时显示鼠标指针
//
// Copyright (C) 2026 Joysaaa1112
// SPDX-License-Identifier: GPL-3.0-or-later
//
// ffmpeg.exe.gz 与 ffmeta.go 由 scripts/build.sh 生成。
package main

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

//go:embed ffmpeg.exe.gz
var ffmpegGz []byte

func init() { runtime.LockOSThread() }

// ================= 中英文 =================

var lang = 0 // 0 中文, 1 English

var tr = map[string][2]string{
	"app":       {"点录", "DotRec"},
	"start":     {"开始录制", "Start recording"},
	"stop":      {"停止并保存", "Stop and save"},
	"preparing": {"准备中…", "Preparing…"},
	"saving":    {"正在保存…", "Saving…"},
	"pause":     {"暂停", "Pause"},
	"resume":    {"继续", "Resume"},
	"elapsed":   {"已录制 %s", "Recorded %s"},
	"pausedAt":  {"已暂停 %s", "Paused at %s"},
	"scope":     {"录制范围", "Area"},
	"sMain":     {"主屏幕", "Main screen"},
	"sAll":      {"全部屏幕", "All screens"},
	"fps":       {"帧率", "Frame rate"},
	"quality":   {"画质", "Quality"},
	"qStd":      {"标准", "Standard"},
	"qHigh":     {"高清（文件更大）", "High (larger file)"},
	"audio":     {"声音", "Audio"},
	"noaudio":   {"不录声音", "No audio"},
	"mouse":     {"显示鼠标指针", "Show cursor"},
	"nohotkey":  {"禁用快捷键", "Disable hotkeys"},
	"openDir":   {"打开保存文件夹", "Open save folder"},
	"openLast":  {"播放最近的录屏", "Play last recording"},
	"language":  {"语言 / Language", "Language / 语言"},
	"exit":      {"退出", "Exit"},
	"tipReady":  {"点录 - 就绪", "DotRec - Ready"},
	"tipPrep":   {"点录 - 准备中", "DotRec - Preparing"},
	"tipRec":    {"点录 - 录制中 %s", "DotRec - Recording %s"},
	"tipPause":  {"点录 - 已暂停 %s", "DotRec - Paused %s"},
	"tipSave":   {"点录 - 正在保存", "DotRec - Saving"},
	"hello":     {"点录在这里。右键图标开始录制，或按 Ctrl+Alt+R。", "DotRec is here. Right-click the icon to record, or press Ctrl+Alt+R."},
	"saved":     {"已保存 %s\n点击这里打开文件夹", "Saved %s\nClick here to open the folder"},
	"hkFail":    {"快捷键 %s 被其他程序占用，请用托盘菜单操作。", "Hotkey %s is used by another app. Use the tray menu instead."},
	"eStart":    {"无法开始录制：", "Could not start recording: "},
	"eResume":   {"无法继续录制：", "Could not resume recording: "},
	"eRec":      {"录制出错，已停止：\n", "Recording failed and was stopped:\n"},
	"eBroke":    {"录制意外中断", "Recording stopped unexpectedly"},
	"eEmpty":    {"没有录到有效画面", "Nothing was recorded"},
	"eMerge":    {"保存视频失败：", "Could not save the video: "},
	"ePrep":     {"准备录制组件失败：", "Could not prepare the recorder: "},
}

func T(k string) string { return tr[k][lang] }

// ================= Win32 =================

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	ole32    = syscall.NewLazyDLL("ole32.dll")
	psapi    = syscall.NewLazyDLL("psapi.dll")

	pRegisterClassExW  = user32.NewProc("RegisterClassExW")
	pCreateWindowExW   = user32.NewProc("CreateWindowExW")
	pDefWindowProcW    = user32.NewProc("DefWindowProcW")
	pGetMessageW       = user32.NewProc("GetMessageW")
	pTranslateMessage  = user32.NewProc("TranslateMessage")
	pDispatchMessageW  = user32.NewProc("DispatchMessageW")
	pPostQuitMessage   = user32.NewProc("PostQuitMessage")
	pPostMessageW      = user32.NewProc("PostMessageW")
	pMessageBoxW       = user32.NewProc("MessageBoxW")
	pLoadImageW        = user32.NewProc("LoadImageW")
	pSetTimer          = user32.NewProc("SetTimer")
	pRegisterHotKey    = user32.NewProc("RegisterHotKey")
	pUnregisterHotKey  = user32.NewProc("UnregisterHotKey")
	pGetSystemMetrics  = user32.NewProc("GetSystemMetrics")
	pDestroyWindow     = user32.NewProc("DestroyWindow")
	pSetForegroundWnd  = user32.NewProc("SetForegroundWindow")
	pSetProcessDPIAwr  = user32.NewProc("SetProcessDPIAware")
	pCreatePopupMenu   = user32.NewProc("CreatePopupMenu")
	pAppendMenuW       = user32.NewProc("AppendMenuW")
	pDestroyMenu       = user32.NewProc("DestroyMenu")
	pTrackPopupMenu    = user32.NewProc("TrackPopupMenu")
	pGetCursorPos      = user32.NewProc("GetCursorPos")
	pRegisterWindowMsg = user32.NewProc("RegisterWindowMessageW")
	pFindWindowW       = user32.NewProc("FindWindowW")

	pCreateMutexW      = kernel32.NewProc("CreateMutexW")
	pGetModuleHandleW  = kernel32.NewProc("GetModuleHandleW")
	pGetCurrentProcess = kernel32.NewProc("GetCurrentProcess")
	pSetWorkingSetSize = kernel32.NewProc("SetProcessWorkingSetSize")
	pEmptyWorkingSet   = psapi.NewProc("EmptyWorkingSet")

	pShellNotifyIconW     = shell32.NewProc("Shell_NotifyIconW")
	pSHGetKnownFolderPath = shell32.NewProc("SHGetKnownFolderPath")
	pCoTaskMemFree        = ole32.NewProc("CoTaskMemFree")
)

const (
	wmNull    = 0x0000
	wmDestroy = 0x0002
	wmClose   = 0x0010
	wmCommand = 0x0111
	wmTimer   = 0x0113
	wmHotkey  = 0x0312
	wmApp     = 0x8000

	wmLButtonUp  = 0x0202
	wmRButtonUp  = 0x0205
	ninBalloonCk = 0x0405

	mfString    = 0x0000
	mfGrayed    = 0x0001
	mfChecked   = 0x0008
	mfPopup     = 0x0010
	mfSeparator = 0x0800
	mftRadio    = 0x0200
	mfDefault   = 0x1000

	msgTray        = wmApp + 1
	msgFfmpegReady = wmApp + 2
	msgSegDied     = wmApp + 3
	msgSaved       = wmApp + 4
	msgSaveFailed  = wmApp + 5
	msgShowMenu    = wmApp + 6

	cmdStartStop = 1
	cmdPause     = 2
	cmdScope0    = 10 // +i
	cmdFps0      = 20
	cmdQual0     = 30
	cmdAudio0    = 100 // +i，0 = 不录声音
	cmdMouse     = 40
	cmdNoHotkey  = 41
	cmdOpenDir   = 42
	cmdOpenLast  = 43
	cmdLang0     = 50
	cmdExit      = 60
)

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type msgT struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	ptX     int32
	ptY     int32
	priv    uint32
}

type notifyIconData struct {
	cbSize           uint32
	hWnd             uintptr
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            uintptr
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uVersion         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         [16]byte
	hBalloonIcon     uintptr
}

func u16(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }
func up(s string) uintptr  { return uintptr(unsafe.Pointer(u16(s))) }
func copyW(dst []uint16, s string) {
	w := syscall.StringToUTF16(s)
	if len(w) > len(dst) {
		w = append(w[:len(dst)-1], 0)
	}
	copy(dst, w)
}
func msgBox(text string, icon uintptr) {
	pMessageBoxW.Call(0, up(text), up(T("app")), icon|0x00010000|0x00040000) // 置前、最上层
}

// ================= 状态与设置 =================

var (
	hInst, hwndMain             uintptr
	icoIdle, icoRec, icoPause   uintptr
	taskbarCreated              uintptr

	ffmpegPath   string
	audioDevices []string

	mu         sync.Mutex
	state      = "idle" // idle | recording | paused | saving
	cur        *segment
	segments   []string
	workDir    string
	accum      time.Duration
	segStart   time.Time
	lastSaved  string
	lastErr    string
	exitAfter  bool
	hotkeysOn  bool
	recCfg     config
	autoSize   bool // 尺寸对不上时改由 FFmpeg 自己识别屏幕
	cfg        = config{fps: 30, mouse: true}
)

type config struct {
	fullDesktop bool
	fps         int
	quality     int
	audio       string
	mouse       bool
	noHotkey    bool
	greeted     bool
}

type segment struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stderr *bytes.Buffer
	done   chan struct{}
	byUs   bool
}

func dataDir() string {
	d := filepath.Join(os.Getenv("LOCALAPPDATA"), "DotRec")
	os.MkdirAll(d, 0755)
	return d
}

func loadConfig() {
	id, _, _ := kernel32.NewProc("GetUserDefaultUILanguage").Call()
	if id&0x3FF == 0x04 {
		lang = 0
	} else {
		lang = 1
	}
	b, err := os.ReadFile(filepath.Join(dataDir(), "settings.txt"))
	if err != nil {
		if l, err := os.ReadFile(filepath.Join(dataDir(), "lang.txt")); err == nil && strings.TrimSpace(string(l)) == "en" {
			lang = 1
		}
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch k {
		case "lang":
			if v == "en" {
				lang = 1
			} else {
				lang = 0
			}
		case "scope":
			cfg.fullDesktop = v == "all"
		case "fps":
			cfg.fps = 30
			if v == "60" {
				cfg.fps = 60
			}
		case "quality":
			cfg.quality = 0
			if v == "high" {
				cfg.quality = 1
			}
		case "audio":
			cfg.audio = v
		case "mouse":
			cfg.mouse = v != "0"
		case "nohotkey":
			cfg.noHotkey = v == "1"
		case "greeted":
			cfg.greeted = v == "1"
		}
	}
}

func saveConfig() {
	b01 := func(b bool) string {
		if b {
			return "1"
		}
		return "0"
	}
	l, sc, q := "zh", "main", "std"
	if lang == 1 {
		l = "en"
	}
	if cfg.fullDesktop {
		sc = "all"
	}
	if cfg.quality == 1 {
		q = "high"
	}
	s := fmt.Sprintf("lang=%s\nscope=%s\nfps=%d\nquality=%s\naudio=%s\nmouse=%s\nnohotkey=%s\ngreeted=%s\n",
		l, sc, cfg.fps, q, cfg.audio, b01(cfg.mouse), b01(cfg.noHotkey), b01(cfg.greeted))
	os.WriteFile(filepath.Join(dataDir(), "settings.txt"), []byte(s), 0644)
}

func trimMemory() {
	debug.FreeOSMemory()
	h, _, _ := pGetCurrentProcess.Call()
	if pEmptyWorkingSet.Find() == nil {
		pEmptyWorkingSet.Call(h)
	} else {
		pSetWorkingSetSize.Call(h, ^uintptr(0), ^uintptr(0))
	}
}

// ================= 托盘 =================

func trayData(flags uint32) *notifyIconData {
	n := &notifyIconData{hWnd: hwndMain, uID: 1, uFlags: flags, uCallbackMessage: msgTray}
	n.cbSize = uint32(unsafe.Sizeof(*n))
	return n
}

func trayAdd() {
	n := trayData(0x01 | 0x02 | 0x04) // MESSAGE | ICON | TIP
	n.hIcon = icoIdle
	copyW(n.szTip[:], T("tipPrep"))
	pShellNotifyIconW.Call(0, uintptr(unsafe.Pointer(n)))
}

func trayUpdate() {
	n := trayData(0x02 | 0x04)
	tip := T("tipReady")
	n.hIcon = icoIdle
	switch {
	case ffmpegPath == "" && state == "idle":
		tip = T("tipPrep")
	case state == "recording":
		tip, n.hIcon = fmt.Sprintf(T("tipRec"), fmtDur(elapsed())), icoRec
	case state == "paused":
		tip, n.hIcon = fmt.Sprintf(T("tipPause"), fmtDur(elapsed())), icoPause
	case state == "saving":
		tip = T("tipSave")
	}
	copyW(n.szTip[:], tip)
	pShellNotifyIconW.Call(1, uintptr(unsafe.Pointer(n)))
}

func trayBalloon(title, body string) {
	n := trayData(0x10) // INFO
	copyW(n.szInfoTitle[:], title)
	copyW(n.szInfo[:], body)
	n.dwInfoFlags = 0x04 | 0x20 // 用程序图标 | 大图标
	pShellNotifyIconW.Call(1, uintptr(unsafe.Pointer(n)))
}

func trayRemove() {
	n := trayData(0)
	pShellNotifyIconW.Call(2, uintptr(unsafe.Pointer(n)))
}

func appendItem(m uintptr, flags uintptr, id int, text string) {
	pAppendMenuW.Call(m, flags, uintptr(id), up(text))
}
func sep(m uintptr) { pAppendMenuW.Call(m, mfSeparator, 0, 0) }
func flag(on bool, f uintptr) uintptr {
	if on {
		return f
	}
	return 0
}

func showMenu() {
	idle := state == "idle"
	lock := flag(!idle, mfGrayed)
	m, _, _ := pCreatePopupMenu.Call()

	switch state {
	case "idle":
		if ffmpegPath == "" {
			appendItem(m, mfGrayed, cmdStartStop, T("preparing"))
		} else {
			appendItem(m, mfDefault, cmdStartStop, T("start")+"\tCtrl+Alt+R")
		}
		appendItem(m, mfGrayed, cmdPause, T("pause")+"\tCtrl+Alt+P")
	case "recording", "paused":
		appendItem(m, mfDefault, cmdStartStop, T("stop")+"\tCtrl+Alt+R")
		if state == "paused" {
			appendItem(m, 0, cmdPause, T("resume")+"\tCtrl+Alt+P")
			appendItem(m, mfGrayed, 0, fmt.Sprintf(T("pausedAt"), fmtDur(elapsed())))
		} else {
			appendItem(m, 0, cmdPause, T("pause")+"\tCtrl+Alt+P")
			appendItem(m, mfGrayed, 0, fmt.Sprintf(T("elapsed"), fmtDur(elapsed())))
		}
	case "saving":
		appendItem(m, mfGrayed, 0, T("saving"))
	}
	sep(m)

	sub := func(title string, opts []string, base int, sel int) {
		s, _, _ := pCreatePopupMenu.Call()
		for i, o := range opts {
			appendItem(s, mftRadio|flag(i == sel, mfChecked)|lock, base+i, o)
		}
		pAppendMenuW.Call(m, mfPopup|lock, s, up(title))
	}
	sc := 0
	if cfg.fullDesktop {
		sc = 1
	}
	sub(T("scope"), []string{T("sMain"), T("sAll")}, cmdScope0, sc)
	fs := 0
	if cfg.fps == 60 {
		fs = 1
	}
	sub(T("fps"), []string{"30 fps", "60 fps"}, cmdFps0, fs)
	sub(T("quality"), []string{T("qStd"), T("qHigh")}, cmdQual0, cfg.quality)
	as := 0
	for i, d := range audioDevices {
		if d == cfg.audio {
			as = i + 1
		}
	}
	sub(T("audio"), append([]string{T("noaudio")}, audioDevices...), cmdAudio0, as)
	appendItem(m, flag(cfg.mouse, mfChecked)|lock, cmdMouse, T("mouse"))
	sep(m)

	appendItem(m, flag(cfg.noHotkey, mfChecked), cmdNoHotkey, T("nohotkey"))
	appendItem(m, 0, cmdOpenDir, T("openDir"))
	appendItem(m, flag(lastSaved == "", mfGrayed), cmdOpenLast, T("openLast"))
	ls, _, _ := pCreatePopupMenu.Call()
	appendItem(ls, mftRadio|flag(lang == 0, mfChecked)|lock, cmdLang0, "中文")
	appendItem(ls, mftRadio|flag(lang == 1, mfChecked)|lock, cmdLang0+1, "English")
	pAppendMenuW.Call(m, mfPopup|lock, ls, up(T("language")))
	sep(m)
	appendItem(m, 0, cmdExit, T("exit"))

	var pt struct{ x, y int32 }
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	pSetForegroundWnd.Call(hwndMain)
	cmd, _, _ := pTrackPopupMenu.Call(m, 0x0100|0x0002|0x0020, uintptr(pt.x), uintptr(pt.y), 0, hwndMain, 0) // RETURNCMD|RIGHTBUTTON|BOTTOMALIGN
	pPostMessageW.Call(hwndMain, wmNull, 0, 0)
	pDestroyMenu.Call(m)
	if cmd != 0 {
		onCommand(int(cmd))
	}
}

func onCommand(cmd int) {
	switch {
	case cmd == cmdStartStop:
		if state == "idle" {
			startRec()
		} else {
			stopRec()
		}
	case cmd == cmdPause:
		togglePause()
	case cmd >= cmdScope0 && cmd < cmdScope0+2:
		cfg.fullDesktop = cmd == cmdScope0+1
	case cmd >= cmdFps0 && cmd < cmdFps0+2:
		cfg.fps = 30
		if cmd == cmdFps0+1 {
			cfg.fps = 60
		}
	case cmd >= cmdQual0 && cmd < cmdQual0+2:
		cfg.quality = cmd - cmdQual0
	case cmd >= cmdAudio0 && cmd <= cmdAudio0+len(audioDevices):
		cfg.audio = ""
		if i := cmd - cmdAudio0; i > 0 {
			cfg.audio = audioDevices[i-1]
		}
	case cmd == cmdMouse:
		cfg.mouse = !cfg.mouse
	case cmd == cmdNoHotkey:
		cfg.noHotkey = !cfg.noHotkey
		applyHotkeys(true)
	case cmd == cmdOpenDir:
		exec.Command("explorer", outputDir()).Start()
	case cmd == cmdOpenLast:
		if lastSaved != "" {
			exec.Command("explorer", lastSaved).Start()
		}
	case cmd == cmdLang0 || cmd == cmdLang0+1:
		lang = cmd - cmdLang0
		trayUpdate()
	case cmd == cmdExit:
		if state == "recording" || state == "paused" {
			exitAfter = true
			stopRec()
		} else if state == "saving" {
			exitAfter = true
		} else {
			pDestroyWindow.Call(hwndMain)
		}
		return
	}
	saveConfig()
}

func applyHotkeys(report bool) {
	pUnregisterHotKey.Call(hwndMain, 1)
	pUnregisterHotKey.Call(hwndMain, 2)
	hotkeysOn = false
	if cfg.noHotkey {
		return
	}
	var failed []string
	if r, _, _ := pRegisterHotKey.Call(hwndMain, 1, 0x0001|0x0002|0x4000, 'R'); r == 0 {
		failed = append(failed, "Ctrl+Alt+R")
	}
	if r, _, _ := pRegisterHotKey.Call(hwndMain, 2, 0x0001|0x0002|0x4000, 'P'); r == 0 {
		failed = append(failed, "Ctrl+Alt+P")
	}
	hotkeysOn = len(failed) < 2
	if report && len(failed) > 0 {
		trayBalloon(T("app"), fmt.Sprintf(T("hkFail"), strings.Join(failed, ", ")))
	}
}

// ================= ffmpeg =================

func prepareFfmpeg() error {
	dir := dataDir()
	path := filepath.Join(dir, "ffmpeg-"+ffVersion+".exe")
	if st, err := os.Stat(path); err == nil && st.Size() == ffSize {
		ffmpegPath = path
		return nil
	}
	zr, err := gzip.NewReader(bytes.NewReader(ffmpegGz))
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	buf := make([]byte, 256<<10)
	if _, err = io.CopyBuffer(f, zr, buf); err != nil {
		f.Close()
		return err
	}
	f.Close()
	if err = os.Rename(tmp, path); err != nil {
		return err
	}
	old, _ := filepath.Glob(filepath.Join(dir, "ffmpeg-*.exe"))
	for _, o := range old {
		if o != path {
			os.Remove(o)
		}
	}
	os.RemoveAll(filepath.Join(os.Getenv("LOCALAPPDATA"), "ScreenRecorder"))
	ffmpegPath = path
	return nil
}

func hidden(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}

var reNew = regexp.MustCompile(`"([^"]+)"\s*\(audio\)`)
var reOld = regexp.MustCompile(`\]\s+"([^"]+)"\s*$`)

func listAudio() []string {
	cmd := exec.Command(ffmpegPath, "-hide_banner", "-list_devices", "true", "-f", "dshow", "-i", "dummy")
	hidden(cmd)
	out, _ := cmd.CombinedOutput()
	var devs []string
	seen := map[string]bool{}
	inAudio := false
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimRight(line, "\r")
		if m := reNew.FindStringSubmatch(line); m != nil {
			if !seen[m[1]] {
				seen[m[1]] = true
				devs = append(devs, m[1])
			}
			continue
		}
		if strings.Contains(line, "DirectShow audio devices") {
			inAudio = true
			continue
		}
		if strings.Contains(line, "DirectShow video devices") {
			inAudio = false
			continue
		}
		if inAudio && !strings.Contains(line, "Alternative name") {
			if m := reOld.FindStringSubmatch(line); m != nil && !seen[m[1]] {
				seen[m[1]] = true
				devs = append(devs, m[1])
			}
		}
	}
	return devs
}

func startSegment() error {
	s := recCfg
	out := filepath.Join(workDir, fmt.Sprintf("part%03d.mkv", len(segments)))
	args := []string{"-hide_banner", "-loglevel", "error", "-y", "-f", "gdigrab", "-framerate", fmt.Sprint(s.fps)}
	if s.mouse {
		args = append(args, "-draw_mouse", "1")
	} else {
		args = append(args, "-draw_mouse", "0")
	}
	if !s.fullDesktop && !autoSize {
		w, _, _ := pGetSystemMetrics.Call(0)
		h, _, _ := pGetSystemMetrics.Call(1)
		args = append(args, "-offset_x", "0", "-offset_y", "0", "-video_size", fmt.Sprintf("%dx%d", w, h))
	}
	args = append(args, "-i", "desktop")
	if s.audio != "" {
		args = append(args, "-f", "dshow", "-audio_buffer_size", "50", "-i", "audio="+s.audio)
	}
	preset, crf := "veryfast", "23"
	if s.fps >= 60 {
		preset = "ultrafast"
	}
	if s.quality == 1 {
		crf = "18"
	}
	args = append(args, "-map", "0:v")
	if s.audio != "" {
		args = append(args, "-map", "1:a", "-c:a", "aac", "-b:a", "160k")
	}
	args = append(args, "-vf", "scale=trunc(iw/2)*2:trunc(ih/2)*2",
		"-c:v", "libx264", "-preset", preset, "-tune", "zerolatency", "-crf", crf,
		"-pix_fmt", "yuv420p", "-threads", "4", out)

	cmd := exec.Command(ffmpegPath, args...)
	hidden(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	seg := &segment{cmd: cmd, stdin: stdin, stderr: &bytes.Buffer{}, done: make(chan struct{})}
	cmd.Stderr = seg.stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	segments = append(segments, out)
	cur = seg
	go func() {
		cmd.Wait()
		close(seg.done)
		mu.Lock()
		byUs := seg.byUs
		mu.Unlock()
		if !byUs {
			pPostMessageW.Call(hwndMain, msgSegDied, 0, 0)
		}
	}()
	return nil
}

func stopSegment() {
	seg := cur
	if seg == nil {
		return
	}
	mu.Lock()
	seg.byUs = true
	mu.Unlock()
	seg.stdin.Write([]byte("q"))
	select {
	case <-seg.done:
	case <-time.After(15 * time.Second):
		seg.cmd.Process.Kill()
		<-seg.done
	}
	cur = nil
}

func videosFolder() string {
	guid := [16]byte{0x1D, 0x9B, 0x98, 0x18, 0xB5, 0x99, 0x5B, 0x45, 0x84, 0x1C, 0xAB, 0x7C, 0x74, 0xE4, 0xDD, 0xFC}
	var p *uint16
	r, _, _ := pSHGetKnownFolderPath.Call(uintptr(unsafe.Pointer(&guid)), 0, 0, uintptr(unsafe.Pointer(&p)))
	if r != 0 || p == nil {
		return ""
	}
	defer pCoTaskMemFree.Call(uintptr(unsafe.Pointer(p)))
	return syscall.UTF16ToString((*[1 << 15]uint16)(unsafe.Pointer(p))[:])
}

func outputDir() string {
	home, _ := os.UserHomeDir()
	var cands []string
	if v := videosFolder(); v != "" {
		cands = append(cands, filepath.Join(v, "DotRec"))
	}
	cands = append(cands, filepath.Join(home, "Videos", "DotRec"), filepath.Join(home, "DotRec"))
	if exe, err := os.Executable(); err == nil {
		cands = append(cands, filepath.Join(filepath.Dir(exe), "DotRec"))
	}
	for _, d := range cands {
		if os.MkdirAll(d, 0755) == nil {
			if st, err := os.Stat(d); err == nil && st.IsDir() {
				return d
			}
		}
	}
	return os.TempDir()
}

func finalize(parts []string, dir string) (string, error) {
	var good []string
	for _, p := range parts {
		if st, err := os.Stat(p); err == nil && st.Size() > 2048 {
			good = append(good, p)
		}
	}
	if len(good) == 0 {
		return "", fmt.Errorf("%s", T("eEmpty"))
	}
	list := filepath.Join(dir, "list.txt")
	var b strings.Builder
	for _, p := range good {
		b.WriteString("file '" + strings.ReplaceAll(filepath.ToSlash(p), "'", `'\''`) + "'\n")
	}
	if err := os.WriteFile(list, []byte(b.String()), 0644); err != nil {
		return "", err
	}
	name := "DotRec_" + time.Now().Format("2006-01-02_15-04-05") + ".mp4"
	out := filepath.Join(outputDir(), name)
	cmd := exec.Command(ffmpegPath, "-hide_banner", "-loglevel", "error", "-y", "-f", "concat", "-safe", "0",
		"-i", list, "-c", "copy", "-movflags", "+faststart", out)
	hidden(cmd)
	if o, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("%s%v\n%s", T("eMerge"), err, tail(string(o)))
	}
	os.RemoveAll(dir)
	return out, nil
}

func tail(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 600 {
		s = "…" + s[len(s)-600:]
	}
	return s
}

// ================= 录制 =================

func startRec() {
	if state != "idle" || ffmpegPath == "" {
		return
	}
	recCfg = cfg
	workDir, _ = os.MkdirTemp("", "dotrec-")
	segments = nil
	autoSize = false
	accum = 0
	if err := startSegment(); err != nil {
		msgBox(T("eStart")+err.Error(), 0x10)
		return
	}
	segStart = time.Now()
	state = "recording"
	trayUpdate()
}

func togglePause() {
	switch state {
	case "recording":
		stopSegment()
		accum += time.Since(segStart)
		state = "paused"
	case "paused":
		if err := startSegment(); err != nil {
			msgBox(T("eResume")+err.Error(), 0x10)
			return
		}
		segStart = time.Now()
		state = "recording"
	default:
		return
	}
	trayUpdate()
}

func stopRec() {
	if state != "recording" && state != "paused" {
		return
	}
	if state == "recording" {
		stopSegment()
		accum += time.Since(segStart)
	}
	state = "saving"
	trayUpdate()
	parts, dir := append([]string(nil), segments...), workDir
	go func() {
		out, err := finalize(parts, dir)
		mu.Lock()
		if err != nil {
			lastErr = err.Error()
			mu.Unlock()
			pPostMessageW.Call(hwndMain, msgSaveFailed, 0, 0)
			return
		}
		lastSaved = out
		mu.Unlock()
		pPostMessageW.Call(hwndMain, msgSaved, 0, 0)
	}()
}

func elapsed() time.Duration {
	if state == "recording" {
		return accum + time.Since(segStart)
	}
	return accum
}

func fmtDur(d time.Duration) string {
	s := int(d.Seconds())
	return fmt.Sprintf("%02d:%02d:%02d", s/3600, s/60%60, s%60)
}

// ================= 窗口消息 =================

func wndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	if taskbarCreated != 0 && msg == taskbarCreated { // 资源管理器重启后重新放图标
		trayAdd()
		trayUpdate()
		return 0
	}
	switch uint32(msg) {
	case msgTray:
		switch uint32(lParam & 0xFFFF) {
		case wmLButtonUp, wmRButtonUp:
			showMenu()
		case ninBalloonCk:
			if lastSaved != "" {
				exec.Command("explorer", "/select,", lastSaved).Start()
			}
		}
		return 0
	case msgShowMenu:
		trayBalloon(T("app"), T("hello"))
		return 0
	case wmHotkey:
		switch wParam {
		case 1:
			onCommand(cmdStartStop)
		case 2:
			togglePause()
		}
		return 0
	case wmTimer:
		if state == "recording" {
			trayUpdate()
		}
		return 0
	case msgFfmpegReady:
		if ffmpegPath == "" {
			msgBox(T("ePrep")+lastErr, 0x10)
		}
		found := cfg.audio == ""
		for _, d := range audioDevices {
			if d == cfg.audio {
				found = true
			}
		}
		if !found {
			cfg.audio = ""
		}
		trayUpdate()
		trimMemory()
		return 0
	case msgSegDied:
		if state == "recording" && cur != nil {
			errText := tail(cur.stderr.String())
			cur = nil
			// 高分屏缩放导致尺寸对不上：去掉指定尺寸，让 FFmpeg 自己识别后立刻重试
			if !autoSize && strings.Contains(errText, "outside window area") {
				autoSize = true
				if n := len(segments); n > 0 {
					os.Remove(segments[n-1])
					segments = segments[:n-1]
				}
				if err := startSegment(); err == nil {
					return 0
				}
			}
			accum += time.Since(segStart)
			if errText == "" {
				errText = T("eBroke")
			}
			state = "paused"
			stopRec()
			msgBox(T("eRec")+errText, 0x10)
		}
		return 0
	case msgSaved:
		state = "idle"
		accum = 0
		trayUpdate()
		if exitAfter {
			pDestroyWindow.Call(hwndMain)
			return 0
		}
		trayBalloon(T("app"), fmt.Sprintf(T("saved"), filepath.Base(lastSaved)))
		trimMemory()
		return 0
	case msgSaveFailed:
		state = "idle"
		trayUpdate()
		msgBox(lastErr, 0x10)
		if exitAfter {
			pDestroyWindow.Call(hwndMain)
		}
		return 0
	case wmClose:
		onCommand(cmdExit)
		return 0
	case wmDestroy:
		trayRemove()
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return r
}

func main() {
	debug.SetGCPercent(20)
	debug.SetMemoryLimit(12 << 20)
	pSetProcessDPIAwr.Call()

	cls := u16("DotRecTray")
	// 只允许运行一个：再次打开时让已在运行的点录弹出提示
	_, _, e := pCreateMutexW.Call(0, 0, up("Local\\DotRecSingleInstance"))
	if e == syscall.Errno(183) { // ERROR_ALREADY_EXISTS
		if w, _, _ := pFindWindowW.Call(uintptr(unsafe.Pointer(cls)), 0); w != 0 {
			pPostMessageW.Call(w, msgShowMenu, 0, 0)
		}
		return
	}

	loadConfig()

	hInst, _, _ = pGetModuleHandleW.Call(0)
	sm := func(i uintptr) uintptr { v, _, _ := pGetSystemMetrics.Call(i); return v }
	cx, cy := sm(49), sm(50) // 小图标尺寸
	load := func(id uintptr) uintptr { h, _, _ := pLoadImageW.Call(hInst, id, 1, cx, cy, 0); return h }
	icoIdle, icoRec, icoPause = load(1), load(2), load(3)

	wc := wndClassEx{lpfnWndProc: syscall.NewCallback(wndProc), hInstance: hInst, lpszClassName: cls, hIcon: icoIdle}
	wc.cbSize = uint32(unsafe.Sizeof(wc))
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	hwndMain, _, _ = pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), up("DotRec"), 0, 0, 0, 0, 0, 0, 0, hInst, 0)
	taskbarCreated, _, _ = pRegisterWindowMsg.Call(up("TaskbarCreated"))

	trayAdd()
	applyHotkeys(true)
	pSetTimer.Call(hwndMain, 1, 1000, 0)
	if !cfg.greeted {
		trayBalloon(T("app"), T("hello"))
		cfg.greeted = true
		saveConfig()
	}

	go func() {
		if err := prepareFfmpeg(); err != nil {
			lastErr = err.Error()
			ffmpegPath = ""
		} else {
			audioDevices = listAudio()
		}
		pPostMessageW.Call(hwndMain, msgFfmpegReady, 0, 0)
	}()

	var m msgT
	for {
		ret, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}
