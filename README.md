# link-launcher

Windows向けのシンプルなリンク・ファイル・フォルダランチャーアプリ。[Wails v2](https://wails.io/)（Go + WebView）で構築されている。

## 特徴

- URL・ファイル・フォルダ・実行ファイルをタイル/リスト表示から起動
- ドラッグ&ドロップでの登録・並び替え
- カテゴリ（タブ）ごとの管理
- ライト/ダーク/システム連動テーマ
- ウィンドウサイズ・表示設定の記憶

## 必要環境

- Windows
- Go 1.23 以上
- [Wails CLI v2](https://wails.io/docs/gettingstarted/installation)

## セットアップ

```bat
go mod tidy
```

## 開発

```bat
wails dev
```

## ビルド

```bat
wails build
```

生成された `build\bin\link-launcher.exe` を実行する。

## 起動オプション

```bat
link-launcher.exe -width 1180 -height 786 -title "リンク集ランチャー"
```

未指定の場合、直前終了時に保存されたウィンドウサイズ（`settings.json`）→ デフォルト値の順で決定される。

## データ保存先

exeと同じディレクトリに以下のファイルを作成する。

- `link-data.json` — 登録したリンク本体
- `settings.json` — ウィンドウサイズ・テーマ・タブ表示設定などのアプリ設定

## ドキュメント

設計上の意図や、機能拡張・保守の際に踏まえておくべき制約・既知の課題は
[`.ClaudeCode/DESIGN.md`](./.ClaudeCode/DESIGN.md) と [`.ClaudeCode/DEV_NOTES.md`](./.ClaudeCode/DEV_NOTES.md) を参照。
