package main

import (
	"flag"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

func main() {
	widthFlag := flag.Int("width", 1180, "ウィンドウ幅")
	heightFlag := flag.Int("height", 786, "ウィンドウ高さ")
	titleFlag := flag.String("title", "リンク集ランチャー", "ウィンドウタイトル")
	flag.Parse()

	// -width / -height が明示的に指定されたかどうかを記録する。
	// 「引数指定＞保存済み設定＞デフォルト値」の優先順位を実現するために必要。
	// (flag.Int の戻り値だけでは「未指定でデフォルト値と同じ」場合と区別できないため)
	widthSet := false
	heightSet := false
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "width":
			widthSet = true
		case "height":
			heightSet = true
		}
	})

	app := NewApp(*widthFlag, *heightFlag, widthSet, heightSet)

	// wails build がバインディング情報生成のため引数なしで一度実行することがある。
	// その場合も os.Exit(1) せずそのまま起動処理に進む（wails.Run内で異常があれば
	// 通常のエラーハンドリングに任せる）。
	err := wails.Run(&options.App{
		Title:  *titleFlag,
		Width:  *widthFlag,
		Height: *heightFlag,
		AssetServer: &assetserver.Options{
			Assets:  nil, // embedしたindex.htmlがHandlerより優先される問題を避けるため常にnil
			Handler: app.assetHandler(),
		},
		BackgroundColour: &options.RGBA{R: 220, G: 224, B: 230, A: 1},
		OnStartup:        app.startup,
		OnBeforeClose:    app.beforeClose,
		// タイトルバー無し(フレームレス)。ドラッグ移動用の領域はフロント側でCSSの
		// --wails-draggable:drag を指定する(.ClaudeCode/DESIGN.md参照)。
		// この2つはWailsのデフォルト値と同じだが、明示しないとruntime:ready時に
		// 空文字で上書きされてしまい、ドラッグ判定が壊れるため必須。
		Frameless:       true,
		CSSDragProperty: "--wails-draggable",
		CSSDragValue:    "drag",
		Bind: []interface{}{
			app,
		},
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     true,
			DisableWebViewDrop: true, // これが無いとWebView2既定の「ドロップ=ダウンロード」挙動と競合する
		},
	})

	if err != nil {
		println("起動エラー:", err.Error())
		os.Exit(1)
	}
}
