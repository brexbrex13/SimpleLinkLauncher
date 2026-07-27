"""クリップボード連携。clipboard_windows.go の PasteClipboardFiles/PasteClipboardImage 相当。

Go版はCF_HDROP/CF_DIBをGDI/Shell APIのsyscall直叩きで手動パースしていたが、Python版は
pywin32(win32clipboard)がCF_HDROPをファイルパスのタプルとして直接返してくれるため
ファイル貼り付け側は大幅に簡潔になる。画像(CF_DIB)側は、生ピクセルを手動でRGBAへ
変換する代わりに、14byteのBITMAPFILEHEADERを付与して標準的なBMPバイト列に変換し、
Pillow(十分に実績のあるBMPデコーダ、BI_BITFIELDS/16・32bit含む)に処理を委譲している。
Go版が自前実装していたビットマスク復元(maskToU8)を再発明しないための意図的な判断。

【重要】Windows専用コードであり、このリポジトリの開発環境(Linux)では実行検証できていない。
実機(Windows)での動作確認が必要(Go版のclipboard_windows.goも同様に
.ClaudeCode/DEV_NOTES.md で実機未検証と明記されている項目)。
"""
import io
import os
import struct
import time

import win32clipboard
from PIL import Image

from pathtype import build_dropped_items


def get_text() -> str:
    """クリップボードのテキストを返す(Wailsランタイムの ClipboardGetText() 相当)。
    テキストが無い場合は空文字を返す。
    """
    win32clipboard.OpenClipboard()
    try:
        if not win32clipboard.IsClipboardFormatAvailable(win32clipboard.CF_UNICODETEXT):
            return ""
        return win32clipboard.GetClipboardData(win32clipboard.CF_UNICODETEXT) or ""
    finally:
        win32clipboard.CloseClipboard()


def paste_clipboard_files() -> list[dict]:
    """クリップボードにファイルオブジェクト(CF_HDROP)があれば種別判定済みリストを返す。
    無ければ空リストを返す(エラーではない。呼び出し側でテキスト貼り付けへフォールバックする)。
    """
    win32clipboard.OpenClipboard()
    try:
        if not win32clipboard.IsClipboardFormatAvailable(win32clipboard.CF_HDROP):
            return []
        paths = win32clipboard.GetClipboardData(win32clipboard.CF_HDROP)
    finally:
        win32clipboard.CloseClipboard()
    return build_dropped_items(list(paths))


def paste_clipboard_image(images_dir: str) -> str:
    """クリップボードの画像(CF_DIB)を images_dir へPNGとして保存し、保存先パスを返す。
    クリップボードに画像が無い場合は空文字を返す(エラーではない)。
    """
    png_bytes = _read_clipboard_image_png()
    if not png_bytes:
        return ""
    os.makedirs(images_dir, exist_ok=True)
    path = os.path.join(images_dir, f"clip_{time.time_ns()}.png")
    with open(path, "wb") as f:
        f.write(png_bytes)
    return path


def _read_clipboard_image_png() -> bytes | None:
    win32clipboard.OpenClipboard()
    try:
        if not win32clipboard.IsClipboardFormatAvailable(win32clipboard.CF_DIB):
            return None
        dib = win32clipboard.GetClipboardData(win32clipboard.CF_DIB)
    finally:
        win32clipboard.CloseClipboard()
    if not dib:
        return None

    bmp_bytes = _dib_to_bmp(bytes(dib))
    with Image.open(io.BytesIO(bmp_bytes)) as img:
        img = img.convert("RGBA")
        buf = io.BytesIO()
        img.save(buf, format="PNG")
        return buf.getvalue()


def _dib_to_bmp(dib: bytes) -> bytes:
    """CF_DIB(BITMAPINFOHEADER以降、ファイルヘッダ無し)の生データに14byteの
    BITMAPFILEHEADERを付与し、Pillowで直接開ける完全なBMPバイト列に変換する。
    24/32bit(BI_RGB)・16/32bit(BI_BITFIELDS)を主な対象とする(Go版と同じ対応範囲)。
    """
    if len(dib) < 40:
        raise ValueError("invalid DIB data")
    bi_size, = struct.unpack_from("<I", dib, 0)
    bi_bit_count, = struct.unpack_from("<H", dib, 14)
    bi_compression, = struct.unpack_from("<I", dib, 16)
    bi_clr_used, = struct.unpack_from("<I", dib, 32)

    if bi_compression == 0:  # BI_RGB
        if bi_bit_count not in (24, 32):
            raise ValueError(f"非対応の色数です: {bi_bit_count}bit")
    elif bi_compression == 3:  # BI_BITFIELDS
        if bi_bit_count not in (16, 32):
            raise ValueError(f"非対応の色数です(BI_BITFIELDS): {bi_bit_count}bit")
    else:
        raise ValueError("非対応の画像形式です(圧縮DIB)")

    off_bits = 14 + bi_size
    if bi_compression == 3 and bi_size == 40:
        # クラシックなBITMAPINFOHEADER(biSize=40)のBI_BITFIELDSは、ヘッダ直後に
        # R/G/Bのカラーマスク3個(12byte)が続き、ピクセルデータはその後ろから始まる。
        # BITMAPV4/V5HEADERではマスク用フィールドが既にbiSizeに含まれているため加算不要。
        off_bits += 12
    elif bi_bit_count <= 8:
        palette_colors = bi_clr_used if bi_clr_used else (1 << bi_bit_count)
        off_bits += palette_colors * 4

    file_size = 14 + len(dib)
    header = struct.pack("<2sIHHI", b"BM", file_size, 0, 0, off_bits)
    return header + dib
