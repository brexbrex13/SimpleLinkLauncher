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

### 配布時の注意（HTMLは同梱されない）

本アプリはフロントエンド（`frontend/link-launcher.html`）をexeにembedせず、実行時に
`exe と同じディレクトリの frontend\link-launcher.html` をディスクから読み込む設計になっている
（理由は [`.ClaudeCode/DESIGN.md`](./.ClaudeCode/DESIGN.md) 参照）。そのため配布時は exe 単体ではなく、
`frontend\link-launcher.html` を同梱する必要がある。

- **`wails build --nsis` でインストーラを作る場合**: `build/windows/installer/project.nsi` で
  `frontend\link-launcher.html` をインストール先へ配置するようにしてあるため、追加作業は不要。
- **exe単体（ポータブル版）を配布する場合**: `build\bin\link-launcher.exe` と同じ階層に
  `frontend\link-launcher.html` を手動でコピーしてから配布する。

## リリース

`v*.*.*` 形式のタグ（例: `v1.0.0`）をpushすると、GitHub Actions
（`.github/workflows/release.yml`）が自動的に以下を行いGitHub Releaseを作成する。

- `wails build --nsis` でexe本体とNSISインストーラをビルド
- `frontend/link-launcher.html` を同梱したポータブル版ZIP（`link-launcher-<tag>-windows-portable.zip`）を作成
- インストーラ（`link-launcher-<tag>-windows-installer.exe`）とあわせてReleaseの資産としてアップロード

```bash
git tag v1.0.0
git push origin v1.0.0
```

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
