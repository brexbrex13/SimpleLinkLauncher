package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math/bits"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

var (
	clipUser32   = syscall.NewLazyDLL("user32.dll")
	clipShell32  = syscall.NewLazyDLL("shell32.dll")
	clipKernel32 = syscall.NewLazyDLL("kernel32.dll")

	procOpenClipboard              = clipUser32.NewProc("OpenClipboard")
	procCloseClipboard             = clipUser32.NewProc("CloseClipboard")
	procGetClipboardData           = clipUser32.NewProc("GetClipboardData")
	procIsClipboardFormatAvailable = clipUser32.NewProc("IsClipboardFormatAvailable")
	procDragQueryFileW             = clipShell32.NewProc("DragQueryFileW")
	procGlobalLock                 = clipKernel32.NewProc("GlobalLock")
	procGlobalUnlock               = clipKernel32.NewProc("GlobalUnlock")
)

const (
	cfHDROP = 15 // CF_HDROP
	cfDIB   = 8  // CF_DIB
)

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

// PasteClipboardImage はクリップボードの画像（CF_DIB形式。スクリーンショット等をコピーした場合）を
// PNGとして exe と同階層の images/ フォルダへ保存し、保存先のフルパスを返す。
// クリップボードに画像が無い場合は空文字を返す（エラーではない）。
//
// 保存したファイルは通常の画像アイテムと同じ扱いにする。つまりアプリ側でアイテムを削除しても
// ファイル自体は削除しない（"file"種別の他アイテムと同様、削除はランチャー登録の解除であって
// 実体の削除ではないという既存の設計方針に揃えている。.ClaudeCode/DESIGN.md参照）。
// そのため専用のクリーンアップ処理は持たない。不要になったファイルは images/ フォルダから
// 手動で削除してもらう想定。
//
// 【重要】GDI APIを直接syscallで叩く自作実装で、実機での動作確認をしていない。
// 24/32bit(BI_RGB)・16/32bit(BI_BITFIELDS、多くのスクリーンショットツールが使う形式)以外の
// DIBフォーマットには対応していない。.ClaudeCode/DEV_NOTES.md参照。
func (a *App) PasteClipboardImage() (string, error) {
	imgPNG, err := readClipboardImagePNG()
	if err != nil {
		return "", err
	}
	if len(imgPNG) == 0 {
		return "", nil
	}

	dir := filepath.Join(a.exeDir, "images")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("clip_%d.png", time.Now().UnixNano()))
	if err := os.WriteFile(path, imgPNG, 0644); err != nil {
		return "", err
	}
	return path, nil
}

func readClipboardImagePNG() ([]byte, error) {
	avail, _, _ := procIsClipboardFormatAvailable.Call(uintptr(cfDIB))
	if avail == 0 {
		return nil, nil
	}

	ret, _, _ := procOpenClipboard.Call(0)
	if ret == 0 {
		return nil, errors.New("OpenClipboard failed")
	}
	defer procCloseClipboard.Call()

	hMem, _, _ := procGetClipboardData.Call(uintptr(cfDIB))
	if hMem == 0 {
		return nil, errors.New("GetClipboardData failed")
	}

	ptr, _, _ := procGlobalLock.Call(hMem)
	if ptr == 0 {
		return nil, errors.New("GlobalLock failed")
	}
	defer procGlobalUnlock.Call(hMem)

	bi := (*bitmapInfoHeader)(unsafe.Pointer(ptr))
	width := int(bi.biWidth)
	height := int(bi.biHeight)
	topDown := height < 0
	if topDown {
		height = -height
	}
	if width <= 0 || height <= 0 {
		return nil, errors.New("invalid clipboard image size")
	}

	headerSize := uintptr(bi.biSize)
	var redMask, greenMask, blueMask uint32
	useMasks := false

	switch bi.biCompression {
	case 0: // BI_RGB（非圧縮）
		if bi.biBitCount != 24 && bi.biBitCount != 32 {
			return nil, fmt.Errorf("非対応の色数です: %dbit", bi.biBitCount)
		}
	case 3: // BI_BITFIELDS。Snipping Tool等の32bitスクリーンショットで一般的な形式
		if bi.biBitCount != 16 && bi.biBitCount != 32 {
			return nil, fmt.Errorf("非対応の色数です(BI_BITFIELDS): %dbit", bi.biBitCount)
		}
		useMasks = true
		// R/G/Bの3個のDWORDカラーマスクは、クラシックなBITMAPINFOHEADER(biSize=40)
		// フィールドの直後(オフセット40)に位置する。BITMAPV4/V5HEADERではこれらのマスク用
		// フィールドがbiSizeに含まれているためオフセットは変わらないが、biSize=40の
		// クラシックな形式ではマスク分(12byte)がbiSizeにカウントされておらず、
		// ピクセルデータの開始位置もその分後ろにずれる。
		maskBase := uintptr(unsafe.Pointer(bi)) + 40
		if bi.biSize == 40 {
			headerSize += 12
		}
		redMask = *(*uint32)(unsafe.Pointer(maskBase))
		greenMask = *(*uint32)(unsafe.Pointer(maskBase + 4))
		blueMask = *(*uint32)(unsafe.Pointer(maskBase + 8))
	default:
		return nil, errors.New("非対応の画像形式です(圧縮DIB)")
	}

	bytesPerPixel := int(bi.biBitCount) / 8
	stride := ((width*int(bi.biBitCount) + 31) / 32) * 4
	pixelsPtr := unsafe.Pointer(uintptr(unsafe.Pointer(bi)) + headerSize)
	pixels := unsafe.Slice((*byte)(pixelsPtr), stride*height)

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		srcY := y
		if !topDown {
			srcY = height - 1 - y // ボトムアップDIBは下から順に格納されている
		}
		rowStart := srcY * stride
		for x := 0; x < width; x++ {
			i := rowStart + x*bytesPerPixel
			var r, g, b, a uint8
			if useMasks {
				var raw uint32
				if bytesPerPixel == 4 {
					raw = uint32(pixels[i]) | uint32(pixels[i+1])<<8 | uint32(pixels[i+2])<<16 | uint32(pixels[i+3])<<24
				} else {
					raw = uint32(pixels[i]) | uint32(pixels[i+1])<<8
				}
				r = maskToU8(raw, redMask)
				g = maskToU8(raw, greenMask)
				b = maskToU8(raw, blueMask)
				a = 255
			} else {
				b = pixels[i]
				g = pixels[i+1]
				r = pixels[i+2]
				a = 255
				if bytesPerPixel == 4 {
					if v := pixels[i+3]; v != 0 {
						a = v // アルファ0埋めのアプリがあるため、0の場合のみ不透明扱いにフォールバックする
					}
				}
			}
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}

	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// maskToU8 はBI_BITFIELDS形式のビットマスクに従い、生のピクセル値から1チャンネル分を
// 8bit値へ正規化して取り出す（マスクの立っているビット幅が8bit未満/超のどちらでも対応）。
func maskToU8(pixel, mask uint32) uint8 {
	if mask == 0 {
		return 0
	}
	shift := bits.TrailingZeros32(mask)
	width := bits.OnesCount32(mask)
	val := (pixel & mask) >> shift
	if width >= 8 {
		return uint8(val >> (width - 8))
	}
	return uint8(val << (8 - width))
}
