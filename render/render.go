package render

import "net/http"

// Render 提供一个通用接口 用于页面渲染
type Render interface {
	Render(w http.ResponseWriter, status int) error
	WriteContentType(w http.ResponseWriter)
}

func writeContentType(w http.ResponseWriter, value string) {
	w.Header().Set("Content-Type", value)
}
