"""パス種別判定・ドロップアイテム構築。app.go の ResolvePathType/buildDroppedItems 相当。"""
import os

# ResolvePathType の画像判定と ReadImageDataURI の両方で使う(app.goのimageMimeTypesと同じ内容)。
IMAGE_MIME_TYPES = {
    ".png": "image/png",
    ".jpg": "image/jpeg",
    ".jpeg": "image/jpeg",
    ".gif": "image/gif",
    ".bmp": "image/bmp",
    ".webp": "image/webp",
    ".ico": "image/x-icon",
}


def resolve_path_type(path: str) -> str:
    """実在するファイル/フォルダのパスから種別("file"|"folder"|"exe"|"image")を判定する。
    存在しないパスの場合は FileNotFoundError を送出する(呼び出し側でURL扱いにフォールバックする想定)。
    """
    if not os.path.exists(path):
        raise FileNotFoundError(path)
    if os.path.isdir(path):
        return "folder"
    ext = os.path.splitext(path)[1].lower()
    if ext in IMAGE_MIME_TYPES:
        return "image"
    if ext == ".exe":
        return "exe"
    return "file"


def build_dropped_items(paths: list[str]) -> list[dict]:
    """パス一覧を種別判定して DroppedItem 相当のdict一覧に変換する。
    存在しない・判定できないパスはスキップする。onFileDrop と PasteClipboardFiles の共通処理。
    """
    items = []
    for p in paths:
        try:
            t = resolve_path_type(p)
        except OSError:
            continue
        base = os.path.basename(p.rstrip("\\/"))
        name = base if t == "folder" else os.path.splitext(base)[0]
        items.append({"name": name, "path": p, "type": t})
    return items
