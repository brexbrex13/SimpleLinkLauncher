# Python版 開発メモ: 移植判断・未検証事項

Go/Wails版の `.ClaudeCode/DEV_NOTES.md` に対応するドキュメント。ここには
「なぜそう移植したか」という判断と、実機(Windows)での動作確認が済んでいない箇所を記録する。

このリポジトリの開発環境はLinuxのため、Windows専用API(pywin32・winreg・os.startfile)を
実際に呼び出すコードパスは一切実行検証できていない。JS側は`window.pywebview.api`を
モックしたヘッドレスChromiumで、ブリッジ層(`window.go.main.App.*`のProxy委譲、
`pywebview-drag-region`クラス付与、`init()`の実行)のみ動作確認済み。

## 依存バージョンの選定(requirements.txt)

`>=`の下限のみを指定し、上限は設けていない。PyPI上の最新版(2026-07時点:
pywebview 6.2.1 / pywin32 312 / Pillow 12.3.0)を実際に調べ、それぞれ本アプリが
使っているAPIが最新メジャーバージョンでも安全に使えることを確認したうえで、
下限をそのメジャーバージョンの先頭(pywebview>=6.0 / Pillow>=12.0)まで引き上げてある。

- **pywebview>=6.0**: 5.x→6.0で`webview.OPEN_DIALOG`/`webview.FOLDER_DIALOG`定数が
  `webview.FileDialog.OPEN`/`webview.FileDialog.FOLDER`enumへ非推奨化されたため、
  `api.py`の`BrowseFile`/`BrowseMultipleFiles`/`BrowseFolder`は新しいenum形式に追従済み。
  本アプリが使うD&D関連API(`webview.dom.DOMEventHandler`、`pywebviewFullPath`)・
  `window.events.closing`・`create_window`の主要引数(`frameless`/`easy_drag`/`js_api`/
  `url`/`x`/`y`/`width`/`height`)は6.0の変更点一覧に含まれておらず、影響なしと判断した。
- **pywin32>=312**: PyPIの最新リリース番号をそのまま下限にした(pywin32はビルド番号のみの
  バージョニングで、306以降はpipでのインストールのみサポートという情報以外に個別APIの
  非推奨情報が見当たらなかったため、深い根拠は無い「最新に追従」の位置づけ)。
- **Pillow>=12.0**: BMP(BI_BITFIELDS含む)デコーダに11→12間の破壊的変更は確認できなかった。

いずれもこのリポジトリの開発環境(Linux)では実際にpywin32・WebView2を動かして検証できて
いないため、実機(Windows)で`pip install -r requirements.txt`した際に問題が出た場合は
このセクションを更新すること。

## 移植方針

- **フロントエンド(`frontend/link-launcher.html`)はほぼ無改変で流用**。Go版との差分は
  実質4箇所のみ(pywebviewブリッジ用のシムスクリプト追加、`--wails-draggable`と併記する
  `-webkit-app-region`のCSS追加、`pywebview-drag-region`クラスの動的付与、
  `DOMContentLoaded`のトリガーを`pyweviewready`待ちでラップ)。詳細は
  `git diff --no-index frontend/link-launcher.html python/frontend/link-launcher.html`
  (リポジトリルートで実行)で確認できる。
- **`api.py`のメソッド名はGo版の`App`構造体と完全一致**させた(`LoadData`/`SaveData`等の
  PascalCase)。これによりフロント側は`window.go.main.App.<Method>` ->
  `window.pywebview.api.<Method>`という1行のProxy委譲だけで済んでいる。
- **アイコン抽出・クリップボードはpywin32(win32gui/win32ui/win32clipboard)に委譲**。
  Go版はGDI/Shell APIをsyscallで直接叩く自作実装だったが、pywin32が同じAPI群を
  ラップしているため生のポインタ演算が不要になり、実装が大幅に簡潔になっている。
- **クリップボード画像(CF_DIB)のデコードはPillowに委譲**。Go版は`maskToU8`という
  ビットマスク復元処理を自作していたが、Python版は14byteのBITMAPFILEHEADERを
  付与して標準的なBMPバイト列に変換し、Pillowの実績あるBMPデコーダ(BI_BITFIELDS対応)に
  処理させている。自前実装を避けることでバグの作り込みリスクを下げる判断。
- **ウィンドウ位置復元の「Wailsバグ用の二重補正」は移植していない**。Go版の
  `app.go:startup()`にある`2*目標値-実際値`という自己補正コードは、vendored wails/v2の
  Windows実装固有の座標系非対称バグ(`WindowGetPosition`は絶対座標、`WindowSetPosition`は
  モニタ相対オフセット加算)へのワークアラウンドであり、pywebviewには同種の変換層が
  無いため理論上は不要と判断した。ただし実機での動作は未確認(下記参照)。
- **起動時のウィンドウサイズ/位置は`create_window()`呼び出し前に解決**。Go版は
  ウィンドウ生成後に`WindowSetSize`/`WindowSetPosition`で事後変更するため「一瞬デフォルト
  サイズで表示されてからリサイズされる」懸念が未検証のまま残っていたが、Python版は
  `main.py`で起動引数/`settings.json`/デフォルト値の優先順位を解決してから
  `webview.create_window(width=, height=, x=, y=)`に渡すため、この問題は構造的に起こらない。

## 実機(Windows)検証が必要な箇所

優先度が高い順。

- **ネイティブファイルドロップ(`main.py`の`DOMEventHandler`)** — pywebview公式Issue #877で
  「ドラッグ元ウィンドウ(Explorer等)がターゲットウィンドウに重なった状態からドラッグを
  開始すると`dragenter`が発火しないことがある」という既知の制約が報告されている。
  実運用(Explorerからこのアプリへドラッグ)でどの程度影響するか要確認。
- **フレームレスウィンドウのドラッグ領域と、ボタン/入力欄のクリック両立
  (`pywebview-drag-region`クラス)** — pywebviewの公式サンプルはドラッグ領域内に
  クリック可能な要素を置くケースを示していないため、`#toolbar`/`#tabBar`内のボタン・
  検索窓が「ドラッグ領域に埋め込まれた状態でも正しくクリック/入力できるか」は未検証。
  Go版でも同種の懸念(`--wails-draggable:drag`と`no-drag`の両立)が
  `.ClaudeCode/DEV_NOTES.md`で実機未検証と明記されている。
- **アイコン抽出(`win_icon.py`)** — `win32ui.CreateDCFromHandle`/`CreateCompatibleDC`/
  `DrawIconEx`/`GetBitmapBits`の組み合わせが実際に正しいBGRA画像を返すか未検証。
  崩れる・例外が出る場合はフロント側が種別グリフへフォールバックする作りだが、
  実機での見た目確認が必要。
- **クリップボード画像貼り付け(`win_clipboard.py`の`_dib_to_bmp`)** — 手組みの
  BITMAPFILEHEADERがPillowで正しく開けるか、特にBI_BITFIELDS(16/32bit)ケースは
  実際のスクリーンショットツール(Snipping Tool等)の出力で確認が必要。
- **クリップボードのファイル貼り付け(`win_clipboard.paste_clipboard_files`)** —
  `win32clipboard.GetClipboardData(CF_HDROP)`がファイルパスのタプルを返す前提で
  実装しているが、実機での戻り値の型・エンコーディングを確認していない。
- **テーマ自動検知のレジストリキー** — Go版と同じ懸念(使用しているWindowsバージョンで
  `AppsUseLightTheme`が存在するか)がそのまま当てはまる。
- **`OpenPath`(`os.startfile`)** — パスにスペース・特殊文字を含む場合の挙動、および
  URL(`https://...`)を渡した場合に既定ブラウザが正しく開くかは未検証
  (Go版の`cmd /c start`と同様の懸念)。
- **pywebviewの初期化タイミング(`whenPywebviewReady`)** — `window.pywebview`が
  `DOMContentLoaded`より前に注入される想定でコードを書いているが、実機のWebView2上で
  本当にそうなっているかは未検証。想定と違う場合でも`pywebviewready`イベント待ちの
  フォールバックがあるため致命的にはならない設計にはしてある。
- **PyInstaller等でexe化した場合の起動速度・サイズ** — 未計測。Go版の単一静的バイナリと
  比べて配布サイズ・初回起動時間が悪化する可能性がある(`BUILD_PYTHON.md`参照)。

## 意図的に移植していない箇所

| 項目 | 対応状況 |
|---|---|
| ウィンドウ位置の二重補正(Wailsのモニタオフセットバグ対策) | 未移植(pywebviewには同種のバグが無い前提のため。上記「移植方針」参照) |
| マルチウィンドウ | 未使用(Go版と同じくシングルウィンドウ構成。画像ビューワもページ内オーバーレイのまま) |
