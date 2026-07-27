"""アイコン抽出。icon_windows.go の ExtractIcon(extractIconPNG) 相当。

Go版はGDI/Shell APIをsyscallで直接叩いていたが、Python版はpywin32(win32com.shell/
win32ui/win32gui)経由で同じAPI群を呼ぶため、生のポインタ演算が不要になり実装が
大幅に簡潔になっている。

【重要】Windows専用コードであり、このリポジトリの開発環境(Linux)では実行検証できていない。
呼び出し手順は実際に動作実績のある公開コード例に合わせてある(SHGetFileInfoは
win32gui ではなく win32com.shell.shell に属する。最初の実装ではこれを誤って
win32gui.SHGetFileInfo としており、実機で `AttributeError` になったため修正した)。
それでも実機(Windows)での動作確認は必要
(Go版のicon_windows.goも同様に .ClaudeCode/DEV_NOTES.md で実機未検証と明記されている項目)。
"""
import base64
import io

import win32api
import win32con
import win32gui
import win32ui
from PIL import Image
from win32com.shell import shell

# SHGetFileInfoW のフラグ。shellcon に無い/確証が薄いため、Go版(icon_windows.go)と
# 同じ値を直接使う。
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
    _ret, info = shell.SHGetFileInfo(path, 0, flags)
    hicon, _i_icon, _dw_attr, _name, _type_name = info
    if not hicon:
        raise OSError(f"アイコンを取得できませんでした: {path}")

    try:
        size = win32api.GetSystemMetrics(win32con.SM_CXICON)
        screen_dc = win32ui.CreateDCFromHandle(win32gui.GetDC(0))
        try:
            mem_bmp = win32ui.CreateBitmap()
            mem_bmp.CreateCompatibleBitmap(screen_dc, size, size)
            mem_dc = screen_dc.CreateCompatibleDC()
            try:
                mem_dc.SelectObject(mem_bmp)
                mem_dc.DrawIcon((0, 0), hicon)
                bmp_info = mem_bmp.GetInfo()
                raw = mem_bmp.GetBitmapBits(True)  # 32bpp BGRA
            finally:
                mem_dc.DeleteDC()
        finally:
            screen_dc.DeleteDC()
    finally:
        win32gui.DestroyIcon(hicon)

    img = Image.frombuffer(
        "RGBA", (bmp_info["bmWidth"], bmp_info["bmHeight"]), raw, "raw", "BGRA", 0, 1
    )
    buf = io.BytesIO()
    img.save(buf, format="PNG")
    return buf.getvalue()
