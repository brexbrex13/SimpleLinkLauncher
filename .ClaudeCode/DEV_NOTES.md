# 開発メモ: 未検証・既知の制約

設計上の意図は [`DESIGN.md`](./DESIGN.md) を参照。ここには実機検証が済んでいない箇所と、
意図的に簡略化・未実装にした箇所を記録する。修正や拡張の際は、まずここに該当項目がないか確認すること。

## 実機（Windows）検証が必要な箇所

優先度が高い順。

- **アイコン抽出（`icon_windows.go`）** — GDI APIを直接syscallで叩いている自作実装。
  `.exe` / フォルダ / 拡張子なしファイルでそれぞれ正しい画像が出るか未検証。
  崩れる・落ちる場合は自作をやめて既存ライブラリへの差し替えを検討する。
- **`OnFileDrop` の座標系・発火可否** — ネイティブDnDとアプリ内並び替え用のHTML5 DnD
  （`draggable`属性）が同一WebView内で共存できるか未検証。共存できない場合、
  内部並び替えの実装方式を作り直す必要がある。
- **ウィンドウリサイズ保存のタイミング** — `window.addEventListener('resize', ...)` が
  Wails WebView上で期待通り発火するか未検証。発火しない場合は `OnBeforeClose` 時のみ
  保存する等の代替手段への変更が必要。
- **起動時のウィンドウサイズ復元** — `app.go` の `startup()` は保存済みサイズをあとから
  `WindowSetSize` で反映する実装のため、一瞬デフォルトサイズで表示されてからリサイズ
  される見え方になっていないか未検証。
- **テーマ自動検知のレジストリキー** — 使用しているWindowsバージョンで
  `AppsUseLightTheme` が存在するか未検証。
- **`cmd /c start` によるURL/ファイルオープン** — パスにスペースや特殊文字が含まれる場合の
  挙動未検証（現状はダブルクォート等の追加処理をしていない）。
- **フレームレスウィンドウ（`main.go`の`Frameless:true`）** — Wailsのソースコード
  （`internal/frontend/desktop/windows/frontend.go`等）を読んで動作原理は確認済みだが、
  実機でのドラッグ移動・端でのリサイズ・Aeroシャドウ/角丸の見た目は未検証。
  `--wails-draggable:drag`(ツールバー全体)と`--wails-draggable:no-drag`(ボタン/入力欄)の
  組み合わせで「ドラッグで移動」と「ボタンのクリック」が両立するかも実機確認が必要。
- **`window.runtime.Quit()`での終了** — `OnBeforeClose`(`SaveWindowSize`)を経由することは
  Wailsのソースコードで確認済みだが、実機での動作(ウィンドウ位置/サイズが次回起動時に
  正しく復元されるか)は未検証。
- **favicon表示（`iconWrapHtml()`の`<img class="favicon">`）** — このサンドボックス開発環境は
  外部ドメインへの実アクセスができない（`curl`でも403）ため、ローカルHTTPサーバーで
  「favicon取得成功」「失敗時のグリフフォールバック」の仕組み自体は確認したが、
  実際のインターネット上のサイトのfaviconでの表示（画質・サイズ・白背景での見え方、
  および取得できないサイトがどの程度あるか）は未検証。目立って崩れる/失敗するサイトが多い
  ようであれば、`PROGRESS.md`フェーズ2の案B（Go側フォールバック）を検討する。
- **クリップボードのファイルオブジェクト貼り付け（`clipboard_windows.go`の`PasteClipboardFiles`）** —
  GDI/Shell APIを直接syscallで叩く自作実装で、実機での動作確認をしていない。特に:
  - `OpenClipboard`が他アプリにロックされている場合の失敗時挙動（リトライ等はしていない）
  - 複数ファイルをコピーした場合の`DragQueryFileW`での列挙順序
  - フォルダ/拡張子なしファイルをコピーした場合の`ResolvePathType`との連携
  うまく動作しない場合は`icon_windows.go`と同様、自作をやめて既存ライブラリへの
  差し替えを検討する。
- **クリップボードの画像貼り付け（`clipboard_windows.go`の`PasteClipboardImage`）** — こちらも
  GDI APIを直接syscallで叩く自作実装で、実機での動作確認をしていない。特に:
  - 24bit/32bit(BI_RGB、非圧縮)以外のDIBフォーマット（16bit、パレット付き8bit以下、圧縮DIB等）は
    未対応でエラーを返す。実際にどのアプリ・操作でどの形式が来るか（Snipping Tool、ブラウザの
    画像コピー、Excel/Wordのコピー等）は未検証。
  - `GlobalLock`で取得したメモリを`unsafe.Pointer`でそのまま解釈しており、`go vet`が
    `possible misuse of unsafe.Pointer`を1件報告する（syscallが返す生アドレスを最初に
    ポインタへ変換する箇所で、Win32メモリ interop では一般的に避けられないパターン。
    実際にはOS管理のメモリで、`OpenClipboard`〜`CloseClipboard`の間で同期的に読むだけなので
    安全と判断しているが、実機での動作で問題が出ないかは確認が必要）。
  - 保存先`images/`フォルダの肥大化は自動クリーンアップしない設計（意図的。`PROGRESS.md`参照）。
    大量に貼り付けた場合の見え方はユーザー側で確認してほしい。
- **`frontend/wailsjs/go/main/App.{js,d.ts}`の手動編集** — `wails`CLIが使えない環境のため、
  `PasteClipboardFiles`・`ReadImageDataURI`・`PasteClipboardImage`のバインディングをこれらの
  自動生成ファイルへ手動で追記した（実際のフロントはこれらを`import`せず
  `window.go.main.App.*`を直接呼ぶ実装のため、動作自体には影響しない）。次回実機で
  `wails build`する際に、本来の自動生成結果とズレていないか確認すること。
- **画像ビューワ・サムネイル（`ReadImageDataURI`、`#imgViewerOverlay`）** — 拡張子ベースの
  MIME判定・Base64変換自体はプラットフォーム非依存の標準ライブラリのみで実装しており
  技術的なリスクは低いが、実機のWebView2上での見た目（大きい画像でのビューワのサイズ感、
  読み込み中の一瞬の空白表示など）は未検証。特に大きな画像ファイルでdata URI変換のコストが
  気になる場合は、キャッシュやサイズ上限の検討が必要になるかもしれない。

## 意図的に簡略化・未実装にした箇所

| 項目 | 対応状況 |
|---|---|
| 空きスロットの見た目 | 破線枠のプレースホルダーで実装（タイル/リスト両方） |
| アイコン自動抽出の実装方法 | Windows API直叩き（`icon_windows.go`）。要検証（上記参照） |
| フルパス取得方法 | Go側 `OnFileDrop` ネイティブAPIを採用（HTML5 DnDのFiles APIは未使用） |
| 設定ファイルの分割 | `link-data.json`（リンク本体）と `settings.json`（設定）に分割。理由は`DESIGN.md`参照 |
| 未分類カテゴリ | カテゴリ名が空文字の場合、見出し非表示のフラット表示として統一 |
| ドラッグ中のリアルタイムインジケーター（外部ドロップ） | 未実装。`OnFileDrop`はドロップ確定時のみ発火するAPIのため、ドラッグ中のプレビュー表示には追加のイベント配線（dragover相当の中継）が必要。現状は内部並び替え時のみ枠線インジケーターあり |
| アイコン変更UI | 簡易版（`prompt()`による絵文字入力のみ）。画像ファイル選択によるカスタムアイコンは未実装 |

## 配布パッケージング

手順は [`BUILD.md`](./BUILD.md) を参照。ここには未検証事項のみ記録する。

`frontend/link-launcher.html` はexeにembedされないため、配布物には別途同梱する必要がある
（詳細は [`DESIGN.md`](./DESIGN.md) の「アセットハンドラ」節を参照）。NSISインストーラ
（`build/windows/installer/project.nsi`）・GitHub Actions（`.github/workflows/release.yml`）
双方とも同梱するようにしてある。

**v0.1.0タグでの初回実行で判明した既知の問題（修正済み）**: `choco install nsis`はマシンの
PATHを更新するが、既に起動済みのGitHub Actionsランナープロセスはそれを再読込しないため、
後続ステップから`makensis`が見つからず、`wails build --nsis`は（エラーにはならず）
警告を出すだけでインストーラexeを生成しない。結果としてポータブルZIPの梱包処理内で
存在しないインストーラexeをコピーしようとして失敗し、ジョブ全体が失敗、
Release作成もスキップされていた。`Install NSIS`ステップで`$GITHUB_PATH`に
`C:\Program Files (x86)\NSIS`を明示的に追記することで解決。再発防止のため、
NSISインストール直後に`makensis -VERSION`で疎通確認するステップと、ポータブルZIP/
インストーラの梱包を別ステップに分離（片方が失敗してももう片方はReleaseに上がる）、
`Create Release`に`if: ${{ !cancelled() }}`を追加。次回タグpush時にActionsの
実行結果を確認すること（インストーラが正しく作られるか、ファイル名が想定通りか等）。
