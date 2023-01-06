package render

import (
	"html/template"
	"net/http"
)

type HTMLRender struct {
	Template *template.Template
}

type HTML struct {
	Template   *template.Template
	Name       string
	Data       any
	IsTemplate bool
}

func (h *HTML) Render(w http.ResponseWriter, status int) error {
	h.WriteContentType(w)
	w.WriteHeader(status)
	if !h.IsTemplate {
		_, err := w.Write([]byte(h.Data.(string)))
		return err
	}
	//使用模板
	err := h.Template.ExecuteTemplate(w, h.Name, h.Data)
	return err
}

func (h *HTML) WriteContentType(w http.ResponseWriter) {
	writeContentType(w, "text/html; charset=utf-8")
}
