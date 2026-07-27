"""アイコン抽出。icon_windows.go の ExtractIcon(extractIconPNG) 相当。

Go版はGDI/Shell APIをsyscallで直接叩いていたが、Python版はpywin32(win32gui/win32ui)経由で
同じAPI群(SHGetFileInfoW/GetIconInfo/DrawIconEx)を呼ぶため、生のポインタ演算が不要になり
実装が大幅に簡潔になっている。

【重要】Windows専用コードであり、このリポジトリの開発環境(Linux)では実行検証できていない。
メソッド名はpywin32の公開ドキュメント・定番レシピに基づくが、実機(Windows)での動作確認が必要
(Go版のicon_windows.goも同様に .ClaudeCode/DEV_NOTES.md で実機未検証と明記されている項目)。
"""
import base64
import io

import win32con
import win32gui
import win32ui
from PIL import Image

# SHGetFileInfoW のフラグ。win32con に定数が無いため、Go版(icon_windows.go)と同じ値を直接使う。
_SHGFI_ICON = 0x000000100
_SHGFI_LARGEICON = 0x000000000


def extract_icon_data_uri(path: str) -> str:
    """指定パスの実ファイル/フォルダ/exeからアイコンを抽出し、
    PNGのBase64データURI("data:image/png;base64,...")として返す。
    取得に失敗した場合は例外を送出する(呼び出し側(JS)はエラー時に種別ごとの
    デフォルトSVGアイコンへフォールバックする)。
    """
    png_bytes = _extract_icon_png(path)
    return "data:image/png;base64," + base64.b64encode(png_bytes).decode("ascii")


def _extract_icon_png(path: str) -> bytes:
    flags = _SHGFI_ICON | _SHGFI_LARGEICON
    _ret, info = win32gui.SHGetFileInfo(path, 0, flags)
    hicon = info[0]
    if not hicon:
        raise OSError(f"アイコンを取得できませんでした: {path}")
    try:
        _f_icon, _hot_x, _hot_y, hbm_mask, hbm_color = win32gui.GetIconInfo(hicon)
        try:
            src_bmp = win32ui.CreateBitmapFromHandle(hbm_color)
            bmp_info = src_bmp.GetInfo()
            width, height = bmp_info["bmWidth"], bmp_info["bmHeight"]
            if width <= 0 or height <= 0:
                raise OSError("invalid icon bitmap size")

            screen_dc = win32ui.CreateDCFromHandle(win32gui.GetDC(0))
            mem_dc = screen_dc.CreateCompatibleDC()
            mem_bmp = win32ui.CreateBitmap()
            mem_bmp.CreateCompatibleBitmap(screen_dc, width, height)
            mem_dc.SelectObject(mem_bmp)
            try:
                win32gui.DrawIconEx(
                    mem_dc.GetSafeHdc(), 0, 0, hicon, width, height, 0, None, win32con.DI_NORMAL
                )
                raw = mem_bmp.GetBitmapBits(True)  # 32bpp BGRA、トップダウン
            finally:
                mem_dc.DeleteDC()
                screen_dc.DeleteDC()
        finally:
            win32gui.DeleteObject(hbm_mask)
            win32gui.DeleteObject(hbm_color)
    finally:
        win32gui.DestroyIcon(hicon)

    img = Image.frombuffer("RGBA", (width, height), raw, "raw", "BGRA", 0, 1)
    buf = io.BytesIO()
    img.save(buf, format="PNG")
    return buf.getvalue()
