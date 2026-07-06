package main

import "golang.org/x/sys/windows/registry"

// GetSystemTheme はWindowsのOS設定（ライト/ダーク）を取得する。
// レジストリ HKCU\...\Personalize\AppsUseLightTheme を読む。取得失敗時は "light" を返す。
// 起動時に一度だけ取得し、OS設定変更へのリアルタイム追従は行わない
// （設計上の理由は .ClaudeCode/DESIGN.md を参照）。
func (a *App) GetSystemTheme() string {
	k, err := registry.OpenKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`,
		registry.QUERY_VALUE,
	)
	if err != nil {
		return "light"
	}
	defer k.Close()

	v, _, err := k.GetIntegerValue("AppsUseLightTheme")
	if err != nil {
		return "light"
	}
	if v == 0 {
		return "dark"
	}
	return "light"
}
