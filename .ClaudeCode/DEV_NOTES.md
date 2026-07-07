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

`frontend/link-launcher.html` はexeにembedされないため、配布物には別途同梱する必要がある
（詳細は [`DESIGN.md`](./DESIGN.md) の「アセットハンドラ」節を参照）。

- NSISインストーラ（`wails build --nsis`）: `build/windows/installer/project.nsi` に
  `frontend\link-launcher.html` を `$INSTDIR\frontend\` へ配置する `File` 命令を追加済み。
  `wails_tools.nsh` は `wails build` のたびに自動生成し直されるため、同梱処理は
  自動生成されない `project.nsi` 側に書いてある。
- ポータブル版（exe単体配布）: `v*.*.*` タグをpushすると `.github/workflows/release.yml` が
  `wails build --nsis` の実行、ポータブルZIPへの同梱、GitHub Releaseへのアップロードまで自動で行う
  （手順はREADMEの「リリース」参照）。ワークフロー自体はWindows実機での
  `wails build --nsis` 成功を前提にしており、CI上での実行結果はまだ確認できていない
  （NSISが `choco install nsis` で問題なく入るか、`link-launcher-amd64-installer.exe`
  というファイル名で出力されるか、等）。初回タグpush時にActionsの実行結果を確認すること。
