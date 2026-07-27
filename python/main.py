"""エントリポイント。main.go 相当。

起動引数 -width/-height/-title は Go版と同じ名前・意味で揃えている
(README.mdの `link-launcher.exe -width 1180 -height 786 -title "..."` という
既存の呼び出し例をそのまま `python main.py -width 1180 ...` に読み替えられるようにするため)。

ウィンドウサイズ・位置の優先順位もGo版(app.go:startup)と同じ:
  起動引数(-width/-height) > 保存済み設定(settings.json) > デフォルト値
ただしPython版はウィンドウ生成前に解決済みの値をcreate_window()へ直接渡せるため、
Go版で懸念されていた「一瞬デフォルトサイズで表示されてからリサイズされる」問題
(.ClaudeCode/DEV_NOTES.md参照)がそもそも起こらない構造になっている。
"""
import argparse
import json
import os
import sys

import webview
from webview.dom import DOMEventHandler

from api import App
from pathtype import build_dropped_items

DEFAULT_WIDTH = 1180
DEFAULT_HEIGHT = 786
DEFAULT_TITLE = "リンク集ランチャー"


def get_exe_dir() -> str:
    if getattr(sys, "frozen", False):
        return os.path.dirname(sys.executable)
    return os.path.dirname(os.path.abspath(__file__))


def load_saved_geometry(settings_path: str) -> dict:
    try:
        with open(settings_path, "r", encoding="utf-8") as f:
            data = json.load(f)
    except (OSError, json.JSONDecodeError):
        return {}
    return data if isinstance(data, dict) else {}


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("-width", type=int, default=None, help="ウィンドウ幅")
    parser.add_argument("-height", type=int, default=None, help="ウィンドウ高さ")
    parser.add_argument("-title", type=str, default=DEFAULT_TITLE, help="ウィンドウタイトル")
    return parser.parse_args()


def _on_drag(_e):
    # dragover等はプレビュー用途。Go版もドラッグ中インジケーターは未実装のため素通しでよい。
    pass


def _on_drop(e, window: "webview.Window"):
    files = (e.get("dataTransfer") or {}).get("files") or []
    paths = [f["pywebviewFullPath"] for f in files if f.get("pywebviewFullPath")]
    if not paths:
        return
    items = build_dropped_items(paths)
    if not items:
        return
    payload = {"x": e.get("clientX", 0), "y": e.get("clientY", 0), "items": items}
    window.evaluate_js(f"window.__pywebviewEmit('files-dropped', {json.dumps(payload)})")


def bind(window: "webview.Window"):
    """ウィンドウ表示後に一度だけ呼ばれる(Go版のstartup(ctx)相当)。ネイティブD&Dを登録する。"""
    window.dom.document.events.dragenter += DOMEventHandler(_on_drag, True, True)
    window.dom.document.events.dragstart += DOMEventHandler(_on_drag, True, True)
    window.dom.document.events.dragover += DOMEventHandler(_on_drag, True, True, debounce=500)
    window.dom.document.events.drop += DOMEventHandler(lambda e: _on_drop(e, window), True, True)


def main():
    args = parse_args()
    exe_dir = get_exe_dir()
    settings_path = os.path.join(exe_dir, "settings.json")
    saved = load_saved_geometry(settings_path)

    width = args.width if args.width is not None else (saved.get("windowWidth") or DEFAULT_WIDTH)
    height = args.height if args.height is not None else (saved.get("windowHeight") or DEFAULT_HEIGHT)
    # ウィンドウ位置は起動引数での指定手段が無い(Go版と同じ)。保存済みがあれば復元する。
    saved_x, saved_y = saved.get("windowX"), saved.get("windowY")
    has_saved_pos = bool(saved_x) or bool(saved_y)

    app = App(exe_dir)

    window_kwargs = dict(
        title=args.title,
        url=app.html_path,
        js_api=app,
        width=width,
        height=height,
        frameless=True,
        easy_drag=False,  # 明示的に pywebview-drag-region クラスを付けた要素のみドラッグ対象にする
        background_color="#DCE0E6",
    )
    if has_saved_pos:
        window_kwargs["x"] = int(saved_x or 0)
        window_kwargs["y"] = int(saved_y or 0)

    window = webview.create_window(**window_kwargs)
    app.set_window(window)

    def on_closing():
        # OnBeforeClose(=SaveWindowSize)を経由してから終了する、というGo版の設計を踏襲。
        app.SaveWindowSize()

    window.events.closing += on_closing

    webview.start(bind, window, debug=False)


if __name__ == "__main__":
    main()
