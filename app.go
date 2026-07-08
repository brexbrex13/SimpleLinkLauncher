package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

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

// imageMimeTypes はサムネイル表示・ビューワ対応する画像拡張子とMIMEタイプの対応表。
// ResolvePathType の画像判定と ReadImageDataURI の両方で使う。
var imageMimeTypes = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".bmp":  "image/bmp",
	".webp": "image/webp",
	".ico":  "image/x-icon",
}

// ResolvePathType は実在するファイル/フォルダのパスから種別("file"|"folder"|"exe"|"image")を判定する。
// URLや存在しないパスの場合はエラーを返す（呼び出し側でURL扱いにフォールバックする想定）。
func (a *App) ResolvePathType(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "folder", nil
	}
	ext := strings.ToLower(filepath.Ext(path))
	if _, ok := imageMimeTypes[ext]; ok {
		return "image", nil
	}
	if ext == ".exe" {
		return "exe", nil
	}
	return "file", nil
}

// ReadImageDataURI は画像ファイルを読み込みBase64データURIとして返す。
// コンパクトビューワでの表示、および一覧でのサムネイルアイコンに使う。
// file://をWebViewに直接読み込ませるとセキュリティ制約に引っかかる可能性があるため、
// 常にGo側で読み込んでdata URI化する（.ClaudeCode/DESIGN.md参照）。
func (a *App) ReadImageDataURI(path string) (string, error) {
	mime, ok := imageMimeTypes[strings.ToLower(filepath.Ext(path))]
	if !ok {
		return "", fmt.Errorf("サポートしていない画像形式です: %s", path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(b), nil
}

var htmlTitleRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

// FetchPageTitle はURLのHTMLを取得し<title>タグの内容を返す。
// クリップボード貼り付け時にURLの項目名として使う（取得失敗時は呼び出し側でパス由来の名前に
// フォールバックする）。favicon取得と同様、CORS制約を避けるためGo側で取得する。
func (a *App) FetchPageTitle(url string) (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("ページの取得に失敗しました: %s", resp.Status)
	}
	// title要素は先頭付近にあるのが通常のため、読みすぎを避け先頭64KBのみ調べる。
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", err
	}
	m := htmlTitleRe.FindSubmatch(body)
	if m == nil {
		return "", fmt.Errorf("titleタグが見つかりませんでした")
	}
	title := html.UnescapeString(string(m[1]))
	title = strings.Join(strings.Fields(title), " ")
	if title == "" {
		return "", fmt.Errorf("titleタグが空でした")
	}
	return title, nil
}

// FileEntry はツリー表示ポップアップ1件分（ディレクトリ直下の子要素）。
type FileEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
}

// ListDirectory はディレクトリ直下の子要素一覧を返す（再帰しない。フロント側が
// ツリーの各ノードを開いたタイミングで都度呼び、遅延展開する）。フォルダを先に、
// 続いてファイルを、それぞれ名前順で返す。
func (a *App) ListDirectory(path string) ([]FileEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	dirs := make([]FileEntry, 0, len(entries))
	files := make([]FileEntry, 0, len(entries))
	for _, e := range entries {
		fe := FileEntry{Name: e.Name(), Path: filepath.Join(path, e.Name()), IsDir: e.IsDir()}
		if e.IsDir() {
			dirs = append(dirs, fe)
		} else {
			files = append(files, fe)
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name < dirs[j].Name })
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return append(dirs, files...), nil
}

// ---- 外部ファイルのドラッグ&ドロップ受信 ----

// DroppedItem はOS側からドロップされたパス1件分の解決結果。
type DroppedItem struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"` // "file" | "folder" | "exe"
}

// buildDroppedItems はパス一覧を種別判定して DroppedItem に変換する。
// 存在しない・判定できないパスはスキップする。onFileDrop と PasteClipboardFiles
// (clipboard_windows.go) の共通処理。
func (a *App) buildDroppedItems(paths []string) []DroppedItem {
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
	return items
}

// onFileDrop はWailsのネイティブOnFileDropから呼ばれる（options.AppにBindしてある）。
// 座標(x,y)とパス一覧をフロントへイベントで転送する。挿入位置の判定はフロント側で行う。
func (a *App) onFileDrop(x, y int, paths []string) {
	items := a.buildDroppedItems(paths)
	if a.ctx == nil || len(items) == 0 {
		return
	}
	runtime.EventsEmit(a.ctx, "files-dropped", map[string]interface{}{
		"x":     x,
		"y":     y,
		"items": items,
	})
}
