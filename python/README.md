# link-launcher (Python版)

link-launcher(Go/Wails版)をPythonで再現した移植版。商用利用可能なpermissiveライセンス
(BSD-3-Clause / PSF License / HPND)のモジュールのみで構成している。

- **状態**: 実装完了・Windows実機での動作確認待ち(このリポジトリの開発環境はLinuxのため、
  D&Dのフルパス取得(`webview.dom`)以外のWindows専用コードパスは未実行検証)。
- **元実装**: リポジトリルートの Go/Wails版(`../app.go`ほか)。機能・データファイル形式
  (`link-data.json`/`settings.json`)は完全互換。

## 構成

- `main.py` — エントリポイント(`../main.go`相当)
- `api.py` — フロントへ公開するAPI(`../app.go`相当)
- `win_icon.py` / `win_theme.py` / `win_clipboard.py` — Windows専用処理
  (`../icon_windows.go` / `../theme_windows.go` / `../clipboard_windows.go`相当)
- `pathtype.py` — パス種別判定の共通ロジック
- `frontend/link-launcher.html` — フロントエンド。Wails版とほぼ無改変
  (差分は`DEV_NOTES_PYTHON.md`参照)

## ドキュメント

- [`BUILD_PYTHON.md`](./BUILD_PYTHON.md) — セットアップ・ビルド・配布手順
- [`DEV_NOTES_PYTHON.md`](./DEV_NOTES_PYTHON.md) — 移植判断の理由、実機未検証の項目一覧

## クイックスタート(Windows)

```bat
cd python
pip install -r requirements.txt
python main.py
```
