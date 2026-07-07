package main

import (
	"errors"
	"syscall"
	"unsafe"
)

var (
	clipUser32  = syscall.NewLazyDLL("user32.dll")
	clipShell32 = syscall.NewLazyDLL("shell32.dll")

	procOpenClipboard              = clipUser32.NewProc("OpenClipboard")
	procCloseClipboard             = clipUser32.NewProc("CloseClipboard")
	procGetClipboardData           = clipUser32.NewProc("GetClipboardData")
	procIsClipboardFormatAvailable = clipUser32.NewProc("IsClipboardFormatAvailable")
	procDragQueryFileW             = clipShell32.NewProc("DragQueryFileW")
)

const cfHDROP = 15 // CF_HDROP

// PasteClipboardFiles はクリップボードにファイルオブジェクト（Explorerで「コピー」した
// ファイル/フォルダ、CF_HDROP形式）がある場合、そのフルパスを種別判定した状態で返す。
// クリップボードにファイルオブジェクトが無い場合（テキストのみ等）は空スライスを返す。
// これはエラーではなく、呼び出し側(JS)はテキスト貼り付け(ClipboardGetText)へフォールバックすること。
//
// 【重要】GDI/Shell APIを直接syscallで叩く自作実装で、実機での動作確認をしていない。
// .ClaudeCode/DEV_NOTES.md 参照。
func (a *App) PasteClipboardFiles() ([]DroppedItem, error) {
	paths, err := readClipboardFilePaths()
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, nil
	}
	return a.buildDroppedItems(paths), nil
}

func readClipboardFilePaths() ([]string, error) {
	avail, _, _ := procIsClipboardFormatAvailable.Call(uintptr(cfHDROP))
	if avail == 0 {
		return nil, nil
	}

	ret, _, _ := procOpenClipboard.Call(0)
	if ret == 0 {
		return nil, errors.New("OpenClipboard failed")
	}
	defer procCloseClipboard.Call()

	hDrop, _, _ := procGetClipboardData.Call(uintptr(cfHDROP))
	if hDrop == 0 {
		return nil, errors.New("GetClipboardData failed")
	}

	// DragQueryFileW(hDrop, 0xFFFFFFFF, nil, 0) はファイル数を返す（Win32の慣例）。
	count, _, _ := procDragQueryFileW.Call(hDrop, 0xFFFFFFFF, 0, 0)
	if count == 0 {
		return nil, nil
	}

	paths := make([]string, 0, count)
	for i := uintptr(0); i < count; i++ {
		size, _, _ := procDragQueryFileW.Call(hDrop, i, 0, 0)
		if size == 0 {
			continue
		}
		buf := make([]uint16, size+1)
		procDragQueryFileW.Call(hDrop, i, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		paths = append(paths, syscall.UTF16ToString(buf))
	}
	return paths, nil
}
