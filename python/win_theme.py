"""Windowsのライト/ダークテーマ検知。theme_windows.go の GetSystemTheme 相当。
標準ライブラリの winreg のみで完結する(追加依存なし)。
"""
import winreg


def get_system_theme() -> str:
    """レジストリ HKCU\\...\\Personalize\\AppsUseLightTheme を読む。取得失敗時は "light" を返す。
    起動時に一度だけ取得し、OS設定変更へのリアルタイム追従は行わない(Go版と同じ設計判断)。
    """
    try:
        with winreg.OpenKey(
            winreg.HKEY_CURRENT_USER,
            r"Software\Microsoft\Windows\CurrentVersion\Themes\Personalize",
        ) as key:
            value, _ = winreg.QueryValueEx(key, "AppsUseLightTheme")
            return "dark" if value == 0 else "light"
    except OSError:
        return "light"
