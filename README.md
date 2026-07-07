# link-launcher

Windows向けのシンプルなリンク・ファイル・フォルダランチャーアプリ。[Wails v2](https://wails.io/)（Go + WebView）で構築されている。

## 特徴

- URL・ファイル・フォルダ・実行ファイル・画像をタイル/リスト表示から起動
- ドラッグ&ドロップ・クリップボード貼り付けでの登録
- カテゴリ（タブ）ごとの管理、URLのfaviconやアイコンの自動表示
- ライト/ダーク/システム連動テーマ
- ウィンドウサイズ・位置・表示設定の記憶

## 動作環境

- Windows 10 / 11
- [Microsoft Edge WebView2 ランタイム](https://developer.microsoft.com/microsoft-edge/webview2/)
  （インストーラ版を使う場合は未導入でも自動でインストールされる）

## 起動オプション

```bat
link-launcher.exe -width 1180 -height 786 -title "リンク集ランチャー"
```

未指定の場合、直前終了時に保存されたウィンドウサイズ・位置（`settings.json`）→ デフォルト値の順で決定される。

## データ保存先

exeと同じディレクトリに以下を作成する。

- `link-data.json` — 登録したリンク本体
- `settings.json` — ウィンドウサイズ・テーマ・タブ表示設定などのアプリ設定
- `images/` — クリップボードから貼り付けた画像の保存先

## サポートについて

もともと作者個人が使うために作ったツールを、そのまま公開しているものです。動作について特に保証はしておりません。
本ソフトウェアに関する質問や、バグや不具合は [Issues](https://github.com/brexbrex13/SimpleLinkLauncher/issues) にご報告ください。

## ライセンス

本ソフトウェアは [MIT License](./LICENSE) で公開しています。商用利用を含め、自由にご利用いただけます。

## サードパーティライセンス

本ソフトウェアは複数のオープンソースソフトウェア（Goモジュール）を使用しています。いずれも商用利用可能な
permissiveライセンス（MIT / BSD / ISC / Apache-2.0 / Unlicense）です。一覧とライセンス全文は
[THIRD_PARTY_LICENSES.md](./THIRD_PARTY_LICENSES.md) を参照してください。

## 開発者向け情報

セットアップ・ビルド・配布・リリース手順や設計上の意図は [`.ClaudeCode/`](./.ClaudeCode/) 以下にまとめています。

- [`.ClaudeCode/BUILD.md`](./.ClaudeCode/BUILD.md) — セットアップ・ビルド・配布・リリース手順
- [`.ClaudeCode/DESIGN.md`](./.ClaudeCode/DESIGN.md) — 設計上の意図・アーキテクチャ
- [`.ClaudeCode/DEV_NOTES.md`](./.ClaudeCode/DEV_NOTES.md) — 実機未検証の項目・既知の制約
