package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App はGo側の窓口関数群。設計方針は .ClaudeCode/DESIGN.md を参照。
type App struct {
	ctx context.Context

	exeDir       string
	htmlPath     string
	dataPath     string
	settingsPath string

	initWidth  int
	initHeight int
	widthSet   bool // -width が起動引数で明示指定されたか
	heightSet  bool // -height が起動引数で明示指定されたか
}

// Settings はexeと同階層に保存するアプリ設定。リンクデータ本体(link-data.json)とは分離する。
type Settings struct {
	WindowWidth   int               `json:"windowWidth"`
	WindowHeight  int               `json:"windowHeight"`
	WindowX       int               `json:"windowX"`
	WindowY       int               `json:"windowY"`
	LastTab       string            `json:"lastTab"`
	Theme         string            `json:"theme"` // "system" | "light" | "dark"
	ViewModeByTab map[string]string `json:"viewModeByTab"`
}

func NewApp(width, height int, widthSet, heightSet bool) *App {
	exeDir := getExeDir()
	return &App{
		exeDir:       exeDir,
		htmlPath:     filepath.Join(exeDir, "frontend", "link-launcher.html"),
		dataPath:     filepath.Join(exeDir, "link-data.json"),
		settingsPath: filepath.Join(exeDir, "settings.json"),
		initWidth:    width,
		initHeight:   height,
		widthSet:     widthSet,
		heightSet:    heightSet,
	}
}

func getExeDir() string {
	exe, err := os.Executable()
	if err != nil {
		wd, _ := os.Getwd()
		return wd
	}
	return filepath.Dir(exe)
}

// assetHandler はfrontend/link-launcher.htmlをリクエストの都度ディスクから読み込んで返す。
// embedを使わないことで、exeを再ビルドせずにHTML/CSS/JSを差し替えられるようにしている。
func (a *App) assetHandler() *fileHandler {
	return &fileHandler{htmlPath: a.htmlPath}
}

// ---- 起動 / 終了 ----

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// ネイティブファイルドロップの受信登録。options.Appのフィールドではなく、
	// ctxを取得できるstartup以降でruntime.OnFileDropを呼ぶ必要がある。
	runtime.OnFileDrop(ctx, a.onFileDrop)

	s, err := a.readSettings()
	if err != nil || s == nil {
		return
	}

	// 引数で明示指定されていないサイズだけ、保存済み設定を優先して反映する。
	// 優先順位: 起動引数 > 保存済み設定 > デフォルト値(main.goのflagデフォルトが既に適用済み)
	if !a.widthSet || !a.heightSet {
		w, h := a.initWidth, a.initHeight
		if !a.widthSet && s.WindowWidth > 0 {
			w = s.WindowWidth
		}
		if !a.heightSet && s.WindowHeight > 0 {
			h = s.WindowHeight
		}
		runtime.WindowSetSize(ctx, w, h)
	}

	// ウィンドウ位置は起動引数での指定手段が無いため、保存済みがあれば常に復元する。
	if s.WindowX != 0 || s.WindowY != 0 {
		runtime.WindowSetPosition(ctx, s.WindowX, s.WindowY)
	}
}

func (a *App) beforeClose(ctx context.Context) bool {
	a.SaveWindowSize()
	return false // false=クローズ処理を継続する
}

// ---- 設定ファイル ----

func (a *App) readSettings() (*Settings, error) {
	b, err := os.ReadFile(a.settingsPath)
	if err != nil {
		return nil, err
	}
	var s Settings
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// LoadSettings はフロントから呼ばれる。ファイルが無ければ空文字を返す（フロント側でデフォルト値を使う）。
func (a *App) LoadSettings() (string, error) {
	b, err := os.ReadFile(a.settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(b), nil
}

func (a *App) SaveSettings(jsonStr string) error {
	return os.WriteFile(a.settingsPath, []byte(jsonStr), 0644)
}

// SaveWindowSize は現在のウィンドウサイズを取得しsettings.jsonへ反映する。
// フロント側のresizeイベント(デバウンス済み)から呼ばれる想定。
func (a *App) SaveWindowSize() error {
	if a.ctx == nil {
		return nil
	}
	w, h := runtime.WindowGetSize(a.ctx)
	x, y := runtime.WindowGetPosition(a.ctx)
	s, err := a.readSettings()
	if err != nil || s == nil {
		s = &Settings{}
	}
	s.WindowWidth = w
	s.WindowHeight = h
	s.WindowX = x
	s.WindowY = y
	if s.ViewModeByTab == nil {
		s.ViewModeByTab = map[string]string{}
	}
	if s.Theme == "" {
		s.Theme = "system"
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.settingsPath, b, 0644)
}

// ---- リンクデータ本体 ----

// LoadData はlink-data.jsonの中身をそのまま文字列で返す。未作成なら空文字（フロント側で初期データを作る）。
func (a *App) LoadData() (string, error) {
	b, err := os.ReadFile(a.dataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(b), nil
}

// SaveData はフロントから渡されたJSON文字列をそのまま保存する。
// データ構造の妥当性チェックはフロント側の責務とする（Go側は単純な書き込み窓口に徹する）。
func (a *App) SaveData(jsonStr string) error {
	return os.WriteFile(a.dataPath, []byte(jsonStr), 0644)
}

// ---- パスを開く ----

// OpenPath はURL/ファイル/フォルダ/exeを既定アプリで開く。Windowsの `start` コマンド相当。
// `cmd /c start` はコマンド自体の起動に成功すればエラーを返さないため、
// ローカルパス（URLではないもの）は事前にos.Statで存在確認してからstartを呼ぶ。
func (a *App) OpenPath(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path is empty")
	}
	if !strings.Contains(path, "://") {
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("パスが見つかりません: %s", path)
		}
	}
	cmd := exec.Command("cmd", "/c", "start", "", path)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Start()
}

// ---- ファイル/フォルダ選択ダイアログ（追加モーダルの「参照…」用） ----

func (a *App) BrowseFile() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "ファイルを選択",
	})
}

// BrowseMultipleFiles は複数ファイルをまとめて選択するダイアログ。
// D&amp;D(OnFileDrop)がWindows上で不安定なため、その代替の主経路として使う。
func (a *App) BrowseMultipleFiles() ([]string, error) {
	return runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "ファイルを選択（複数選択可）",
	})
}

func (a *App) BrowseFolder() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "フォルダを選択",
	})
}

// ResolvePathType は実在するファイル/フォルダのパスから種別("file"|"folder"|"exe")を判定する。
// URLや存在しないパスの場合はエラーを返す（呼び出し側でURL扱いにフォールバックする想定）。
func (a *App) ResolvePathType(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "folder", nil
	}
	if strings.EqualFold(filepath.Ext(path), ".exe") {
		return "exe", nil
	}
	return "file", nil
}

// ---- 外部ファイルのドラッグ&ドロップ受信 ----

// DroppedItem はOS側からドロップされたパス1件分の解決結果。
type DroppedItem struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"` // "file" | "folder" | "exe"
}

// onFileDrop はWailsのネイティブOnFileDropから呼ばれる（options.AppにBindしてある）。
// 座標(x,y)とパス一覧をフロントへイベントで転送する。挿入位置の判定はフロント側で行う。
func (a *App) onFileDrop(x, y int, paths []string) {
	items := make([]DroppedItem, 0, len(paths))
	for _, p := range paths {
		t, err := a.ResolvePathType(p)
		if err != nil {
			continue
		}
		base := filepath.Base(p)
		name := base
		if t != "folder" {
			name = strings.TrimSuffix(base, filepath.Ext(base))
		}
		items = append(items, DroppedItem{Name: name, Path: p, Type: t})
	}
	if a.ctx == nil || len(items) == 0 {
		return
	}
	runtime.EventsEmit(a.ctx, "files-dropped", map[string]interface{}{
		"x":     x,
		"y":     y,
		"items": items,
	})
}
