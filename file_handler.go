package main

import (
	"net/http"
	"os"
)

// fileHandler はAssetServer.Handlerとして使う。
// 毎リクエスト os.ReadFile するので、exeを再ビルドせずにHTMLを編集して即反映できる。
// アクセス数が多いアプリではないため、キャッシュはあえて実装しない（可読性優先）。
type fileHandler struct {
	htmlPath string
}

func (h *fileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile(h.htmlPath)
	if err != nil {
		http.Error(w, "frontend not found: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}
