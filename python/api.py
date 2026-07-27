"""JS側(window.pywebview.api.*)に公開するAPIクラス。app.go の App 相当。

設計方針はGo版の .ClaudeCode/DESIGN.md を踏襲する: ここは単純な窓口に徹し、
並び替え・カテゴリ分類等のビジネスロジックは持ち込まない(フロントJS側の責務)。

メソッド名はGo版とあえて完全に一致させている(LoadData/SaveData等のPascalCase)。
これによりフロント側(frontend/link-launcher.html)は
`window.go.main.App.<Method>` -> `window.pywebview.api.<Method>` という
1行のブリッジで済み、既存のロジックをほぼ無改変で流用できる。
"""
import base64
import html
import json
import os
import re
import urllib.error
import urllib.request

import webview

from pathtype import IMAGE_MIME_TYPES, resolve_path_type

_TITLE_RE = re.compile(r"<title[^>]*>(.*?)</title>", re.IGNORECASE | re.DOTALL)


def _default_settings() -> dict:
    return {
        "windowWidth": 0,
        "windowHeight": 0,
        "windowX": 0,
        "windowY": 0,
        "lastTab": "",
        "theme": "system",
        "viewModeByTab": {},
    }


class App:
    def __init__(self, exe_dir: str):
        self.exe_dir = exe_dir
        self.html_path = os.path.join(exe_dir, "frontend", "link-launcher.html")
        self.data_path = os.path.join(exe_dir, "link-data.json")
        self.settings_path = os.path.join(exe_dir, "settings.json")
        self.images_dir = os.path.join(exe_dir, "images")
        self._window = None

    def set_window(self, window: "webview.Window") -> None:
        self._window = window

    # ---- 設定ファイル ----

    def _read_settings(self) -> dict | None:
        try:
            with open(self.settings_path, "r", encoding="utf-8") as f:
                return json.load(f)
        except FileNotFoundError:
            return None
        except (OSError, json.JSONDecodeError):
            return None

    def LoadSettings(self) -> str:
        """ファイルが無ければ空文字を返す(フロント側でデフォルト値を使う)。"""
        try:
            with open(self.settings_path, "r", encoding="utf-8") as f:
                return f.read()
        except FileNotFoundError:
            return ""

    def SaveSettings(self, json_str: str) -> None:
        with open(self.settings_path, "w", encoding="utf-8") as f:
            f.write(json_str)

    def SaveWindowSize(self) -> None:
        """現在のウィンドウサイズ・位置を取得しsettings.jsonへ反映する。
        フロント側のresizeイベント(デバウンス済み)から呼ばれる想定。
        """
        if self._window is None:
            return
        s = self._read_settings() or _default_settings()
        s["windowWidth"] = int(self._window.width)
        s["windowHeight"] = int(self._window.height)
        s["windowX"] = int(self._window.x)
        s["windowY"] = int(self._window.y)
        if not isinstance(s.get("viewModeByTab"), dict):
            s["viewModeByTab"] = {}
        if not s.get("theme"):
            s["theme"] = "system"
        with open(self.settings_path, "w", encoding="utf-8") as f:
            json.dump(s, f, ensure_ascii=False, indent=2)

    # ---- リンクデータ本体 ----

    def LoadData(self) -> str:
        try:
            with open(self.data_path, "r", encoding="utf-8") as f:
                return f.read()
        except FileNotFoundError:
            return ""

    def SaveData(self, json_str: str) -> None:
        with open(self.data_path, "w", encoding="utf-8") as f:
            f.write(json_str)

    # ---- パスを開く ----

    def OpenPath(self, path: str) -> None:
        """URL/ファイル/フォルダ/exeを既定アプリで開く。os.startfile (ShellExecute相当)を使う。"""
        if not path or not path.strip():
            raise ValueError("path is empty")
        if "://" not in path:
            if not os.path.exists(path):
                raise FileNotFoundError(f"パスが見つかりません: {path}")
        os.startfile(path)  # Windows専用

    # ---- ファイル/フォルダ選択ダイアログ ----

    def BrowseFile(self) -> str:
        result = self._window.create_file_dialog(webview.OPEN_DIALOG, allow_multiple=False)
        return result[0] if result else ""

    def BrowseMultipleFiles(self) -> list[str]:
        result = self._window.create_file_dialog(webview.OPEN_DIALOG, allow_multiple=True)
        return list(result) if result else []

    def BrowseFolder(self) -> str:
        result = self._window.create_file_dialog(webview.FOLDER_DIALOG)
        return result[0] if result else ""

    # ---- パス種別判定・画像 ----

    def ResolvePathType(self, path: str) -> str:
        return resolve_path_type(path)

    def ReadImageDataURI(self, path: str) -> str:
        ext = os.path.splitext(path)[1].lower()
        mime = IMAGE_MIME_TYPES.get(ext)
        if not mime:
            raise ValueError(f"サポートしていない画像形式です: {path}")
        with open(path, "rb") as f:
            data = f.read()
        return f"data:{mime};base64," + base64.b64encode(data).decode("ascii")

    # ---- URLタイトル取得 ----

    def FetchPageTitle(self, url: str) -> str:
        req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
        try:
            with urllib.request.urlopen(req, timeout=5) as resp:
                if resp.status < 200 or resp.status >= 300:
                    raise ValueError(f"ページの取得に失敗しました: {resp.status}")
                body = resp.read(64 * 1024)
                charset = resp.headers.get_content_charset() or "utf-8"
        except urllib.error.URLError as e:
            raise ValueError(f"ページの取得に失敗しました: {e}") from e

        text = body.decode(charset, errors="ignore")
        m = _TITLE_RE.search(text)
        if not m:
            raise ValueError("titleタグが見つかりませんでした")
        title = html.unescape(m.group(1))
        title = " ".join(title.split())
        if not title:
            raise ValueError("titleタグが空でした")
        return title

    # ---- フォルダツリー表示 ----

    def ListDirectory(self, path: str) -> list[dict]:
        dirs, files = [], []
        for name in os.listdir(path):
            full = os.path.join(path, name)
            entry = {"name": name, "path": full, "isDir": os.path.isdir(full)}
            (dirs if entry["isDir"] else files).append(entry)
        dirs.sort(key=lambda e: e["name"])
        files.sort(key=lambda e: e["name"])
        return dirs + files

    # ---- クリップボード貼り付け画像の後始末 ----

    def _managed_image_path(self, path: str) -> str:
        images_dir = os.path.abspath(self.images_dir)
        abs_path = os.path.abspath(path)
        try:
            rel = os.path.relpath(abs_path, images_dir)
        except ValueError:
            return ""
        if rel == "." or rel.startswith(".."):
            return ""
        return abs_path

    def IsManagedImage(self, path: str) -> bool:
        return self._managed_image_path(path) != ""

    def DeleteManagedImage(self, path: str) -> None:
        managed = self._managed_image_path(path)
        if not managed:
            return
        os.remove(managed)

    # ---- アイコン抽出 (win_icon.py へ委譲) ----

    def ExtractIcon(self, path: str) -> str:
        from win_icon import extract_icon_data_uri

        return extract_icon_data_uri(path)

    # ---- テーマ検知 (win_theme.py へ委譲) ----

    def GetSystemTheme(self) -> str:
        from win_theme import get_system_theme

        return get_system_theme()

    # ---- クリップボード貼り付け (win_clipboard.py へ委譲) ----

    def PasteClipboardFiles(self) -> list[dict]:
        from win_clipboard import paste_clipboard_files

        return paste_clipboard_files()

    def PasteClipboardImage(self) -> str:
        from win_clipboard import paste_clipboard_image

        return paste_clipboard_image(self.images_dir)

    def ClipboardGetText(self) -> str:
        """window.runtime.ClipboardGetText()(Wailsランタイム)のシム先。win_clipboard.get_textに委譲。"""
        from win_clipboard import get_text

        return get_text()

    # ---- 終了 ----

    def Quit(self) -> None:
        """OnBeforeClose(=SaveWindowSize)を経由してから終了する(main.pyのclosingハンドラ参照)。"""
        if self._window is not None:
            self._window.destroy()
