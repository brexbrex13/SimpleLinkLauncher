package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/png"
	"syscall"
	"unsafe"
)

// ExtractIcon は指定パスの実ファイル/フォルダ/exeからアイコンを抽出し、
// PNGのBase64データURI（"data:image/png;base64,..."）として返す。
// 取得に失敗した場合はエラーを返す。呼び出し側(JS)はエラー時に種別ごとの
// デフォルトSVGアイコンへフォールバックすること。
//
// GDI/Shell APIを直接syscallで叩く自作実装。設計上の理由と未検証の項目は
// .ClaudeCode/DESIGN.md, .ClaudeCode/DEV_NOTES.md を参照。
func (a *App) ExtractIcon(path string) (string, error) {
	b, err := extractIconPNG(path)
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(b), nil
}

var (
	shell32 = syscall.NewLazyDLL("shell32.dll")
	user32  = syscall.NewLazyDLL("user32.dll")
	gdi32   = syscall.NewLazyDLL("gdi32.dll")

	procSHGetFileInfoW     = shell32.NewProc("SHGetFileInfoW")
	procDestroyIcon        = user32.NewProc("DestroyIcon")
	procGetIconInfo        = user32.NewProc("GetIconInfo")
	procGetObject          = gdi32.NewProc("GetObjectW")
	procGetDIBits          = gdi32.NewProc("GetDIBits")
	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procDeleteDC           = gdi32.NewProc("DeleteDC")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
	procSelectObject       = gdi32.NewProc("SelectObject")
)

const (
	shgfiIcon      = 0x000000100
	shgfiLargeIcon = 0x000000000
	dibRgbColors   = 0
)

type shfileinfoW struct {
	hIcon         syscall.Handle
	iIcon         int32
	dwAttributes  uint32
	szDisplayName [260]uint16
	szTypeName    [80]uint16
}

type iconInfo struct {
	fIcon    int32
	xHotspot uint32
	yHotspot uint32
	hbmMask  syscall.Handle
	hbmColor syscall.Handle
}

type bitmap struct {
	bmType       int32
	bmWidth      int32
	bmHeight     int32
	bmWidthBytes int32
	bmPlanes     uint16
	bmBitsPixel  uint16
	bmBits       uintptr
}

type bitmapInfoHeader struct {
	biSize          uint32
	biWidth         int32
	biHeight        int32
	biPlanes        uint16
	biBitCount      uint16
	biCompression   uint32
	biSizeImage     uint32
	biXPelsPerMeter int32
	biYPelsPerMeter int32
	biClrUsed       uint32
	biClrImportant  uint32
}

func extractIconPNG(path string) ([]byte, error) {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}

	var info shfileinfoW
	ret, _, _ := procSHGetFileInfoW.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0,
		uintptr(unsafe.Pointer(&info)),
		unsafe.Sizeof(info),
		uintptr(shgfiIcon|shgfiLargeIcon),
	)
	if ret == 0 || info.hIcon == 0 {
		return nil, errors.New("SHGetFileInfo failed")
	}
	hIcon := info.hIcon
	defer procDestroyIcon.Call(uintptr(hIcon))

	var ii iconInfo
	ret, _, _ = procGetIconInfo.Call(uintptr(hIcon), uintptr(unsafe.Pointer(&ii)))
	if ret == 0 {
		return nil, errors.New("GetIconInfo failed")
	}
	defer procDeleteObject.Call(uintptr(ii.hbmMask))
	defer procDeleteObject.Call(uintptr(ii.hbmColor))

	var bmp bitmap
	procGetObject.Call(uintptr(ii.hbmColor), unsafe.Sizeof(bmp), uintptr(unsafe.Pointer(&bmp)))
	w := int(bmp.bmWidth)
	h := int(bmp.bmHeight)
	if w <= 0 || h <= 0 {
		return nil, errors.New("invalid icon bitmap size")
	}

	hdc, _, _ := procCreateCompatibleDC.Call(0)
	if hdc == 0 {
		return nil, errors.New("CreateCompatibleDC failed")
	}
	defer procDeleteDC.Call(hdc)

	oldObj, _, _ := procSelectObject.Call(hdc, uintptr(ii.hbmColor))
	defer procSelectObject.Call(hdc, oldObj)

	bi := bitmapInfoHeader{
		biSize:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		biWidth:       int32(w),
		biHeight:      int32(-h), // 負数=トップダウン(上から順)で受け取る
		biPlanes:      1,
		biBitCount:    32,
		biCompression: 0, // BI_RGB
	}

	buf := make([]byte, w*h*4)
	ret, _, _ = procGetDIBits.Call(
		hdc,
		uintptr(ii.hbmColor),
		0,
		uintptr(h),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&bi)),
		dibRgbColors,
	)
	if ret == 0 {
		return nil, errors.New("GetDIBits failed")
	}

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			bch := buf[i]
			gch := buf[i+1]
			rch := buf[i+2]
			ach := buf[i+3]
			img.Set(x, y, color.RGBA{R: rch, G: gch, B: bch, A: ach})
		}
	}

	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
