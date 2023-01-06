package render

import (
	"encoding/xml"
	"net/http"
)

type XML struct {
	Data any
}

var xmlContentType = []string{"application/xml; charset=utf-8"}

func (x *XML) Render(w http.ResponseWriter, status int) error {
	x.WriteContentType(w)
	w.WriteHeader(status)
	return xml.NewEncoder(w).Encode(x.Data)
}

func (x *XML) WriteContentType(w http.ResponseWriter) {
	writeContentType(w, xmlContentType[0])
}
