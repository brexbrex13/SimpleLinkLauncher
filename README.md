# link-launcher セットアップ手順

このリポジトリ一式は **Windows実機でのビルド・動作確認を一切していません**。
コードとして書けるところまで書いていますが、以下の手順とチェックリストに沿って
必ず実機で検証してください。

## 1. セットアップ手順

```bat
cd C:\Files\Programing\go-lang\projects
wails init -n link-launcher -t vanilla
```

生成された `link-launcher` フォルダの中身を、このセッションで作成した以下のファイルで置き換える。

```
main.go            ← 置き換え
app.go              ← 追加
file_handler.go     ← 追加
theme_windows.go    ← 追加
icon_windows.go      ← 追加
wails.json          ← 置き換え
frontend\           ← 中身を削除し、link-launcher.html のみ配置
```

`frontend` フォルダ内の `wailsjs` 等の自動生成物・`package.json` は不要なら削除してよい
（`frontend:install` / `frontend:build` を空にしているため npm は使わない）。

```bat
go mod tidy
```

`golang.org/x/sys/windows/registry`（テーマ検知）を使うため、依存が自動追加される。

```bat
wails build
```

生成された `build\bin\link-launcher.exe` を実行。

## 2. 実機検証チェックリスト（未検証・要確認）

優先度高い順。

- [ ] **アイコン抽出 (`icon_windows.go`)** — GDI APIを直接syscallで叩いている自作実装。
      `.exe` / フォルダ / 拡張子なしファイルでそれぞれ正しい画像が出るか。
      崩れる・落ちるようなら自作をやめて既存ライブラリへの差し替えを検討。
- [ ] **`OnFileDrop` の座標系・発火可否** — ネイティブDnDとアプリ内並び替え用の
      HTML5 DnD（`draggable`属性）が同一WebView内で共存できるか。
      共存できない場合、内部並び替えの実装方式を作り直す必要あり。
- [ ] **ウィンドウリサイズ保存のタイミング** — `window.addEventListener('resize', ...)`
      がWails WebView上で期待通り発火するか。発火しない場合は代替手段
      （`OnBeforeClose`時のみ保存、等）に変更する。
- [ ] **起動時のウィンドウサイズ復元** — 一瞬デフォルトサイズで表示されてから
      リサイズされる見え方になっていないか（`app.go` の `startup()` 参照）。
- [ ] **テーマ自動検知のレジストリキー** — 使用しているWindowsバージョンで
      `AppsUseLightTheme` が存在するか。
- [ ] **`cmd /c start` によるURL/ファイルオープン** — パスにスペースや特殊文字が
      含まれる場合の挙動（現状はダブルクォート等の追加処理をしていない）。

## 3. 既知の未実装・簡略化した箇所（申し送り事項の対応状況）

| 申し送り事項 | 対応状況 |
|---|---|
| 空きスロットの見た目 | 破線枠のプレースホルダーで実装（タイル/リスト両方） |
| アイコン自動抽出の実装方法 | Windows API直叩き（`icon_windows.go`）で実装。要検証 |
| フルパス取得方法 | Go側 `OnFileDrop` ネイティブAPIを採用（HTML5 DnDのFiles APIは未使用） |
| 設定ファイルの分割 | `link-data.json`（リンク本体）と `settings.json`（設定）に分割 |
| 未分類カテゴリ | カテゴリ名が空文字の場合、見出し非表示のフラット表示として統一 |
| ドラッグ中のリアルタイムインジケーター（外部ドロップ） | **未実装**。`OnFileDrop`はドロップ確定時のみ発火するAPIのため、
ドラッグ中のプレビュー表示には追加のイベント配線（dragover相当の中継）が必要。現状は内部並び替え時のみ枠線インジケーターあり |
| アイコン変更UI | 簡易版（`prompt()`による絵文字入力のみ）。画像ファイル選択によるカスタムアイコンは未実装 |
