# Python版 ビルド・開発・リリース手順

これは `python/` 配下にある link-launcher の Python(pywebview)移植版のビルド手順。
Go/Wails版の `.ClaudeCode/BUILD.md` に対応するドキュメント。設計・移植判断の詳細は
[`DEV_NOTES_PYTHON.md`](./DEV_NOTES_PYTHON.md) を参照。

## セットアップ

Windows上で(pywin32・WebView2依存のため):

```bat
cd python
python -m venv venv
venv\Scripts\activate
pip install -r requirements.txt
```

[Microsoft Edge WebView2 ランタイム](https://developer.microsoft.com/microsoft-edge/webview2/)
が必要(Go/Wails版と同じ前提。Windows 10/11では標準搭載されていることが多い)。

## 開発

```bat
python main.py
```

`frontend/link-launcher.html` は実行のたびにディスクから読み込まれるため、
アプリ再起動なしでHTML/CSS/JSを編集して確認できる(Go版のfile_handler.goと同じ設計意図)。

起動オプションはGo版と同じ形式:

```bat
python main.py -width 1180 -height 786 -title "リンク集ランチャー"
```

## ビルド(配布用exe化)

PyInstallerを使う場合:

```bat
pip install pyinstaller
pyinstaller --noconfirm --windowed --onefile --name link-launcher ^
  --icon ..\build\appicon.ico ^
  main.py
```

生成される `dist\link-launcher.exe` を実行する。

### 配布時の注意(HTMLは同梱されない)

Go版と同じ理由(exeを再ビルドせずにHTML/CSS/JSを差し替えられるようにするため)で、
`frontend/link-launcher.html` は onefile exe に埋め込まない。配布時は

```
dist\link-launcher.exe
dist\frontend\link-launcher.html   ← 手動でコピー
```

の構成にする。

### PyInstaller以外の選択肢

PyInstallerの核はGPLv2ライセンスだが、生成した配布物(あなたのアプリのexe)は
ブートローダ例外により閉じたまま配布してよい(商用利用に問題なし)。ライセンスの
議論自体を避けたい場合は以下も選択肢:

- **cx_Freeze**(MITライセンス)
- **Nuitka**(Apache-2.0ライセンス。Pythonをコンパイルするため起動が速い傾向)

いずれもコマンド例は各ツールの公式ドキュメント参照。exeとHTMLを分離する配布方針は変わらない。

## リリース

Go版と同様、Windows実機でビルドしたexeをNSISインストーラやポータブルZIPにまとめて配布する
運用を想定。GitHub Actionsで自動化する場合、Go版の `.github/workflows/release.yml` の
`windows-latest` ランナー構成を流用し、`wails build` の部分を上記PyInstallerコマンドに
置き換える形になる(このセッションでは未実装。Python版を正式運用する場合は別途整備が必要)。

補足: PythonでWindows向けexeを作るには実際にWindows環境(ローカル機かGitHub Actionsの
`windows-latest`)が必要。Goの `GOOS=windows go build` のような単純なクロスコンパイルは
PyInstaller/cx_Freeze/Nuitkaのいずれでもできない。
