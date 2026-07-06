package main

import "golang.org/x/sys/windows/registry"

// GetSystemTheme はWindowsのOS設定（ライト/ダーク）を取得する。
// レジストリ HKCU\...\Personalize\AppsUseLightTheme を読む。取得失敗時は "light" を返す。
//
// 要検証: Windowsのバージョンによってはキー自体が存在しない場合がある。
// また「起動時に1回取得するだけ」で、OS設定をリアルタイム追従はしない
// （追従させるならレジストリの変更通知(RegNotifyChangeKeyValue)が必要になるが、
//
//	今回のスコープでは過剰と判断し実装していない）。
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
