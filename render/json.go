package render

import (
	"encoding/json"
	"net/http"
)

type JSON struct {
	Data any
}

var jsonContentType = []string{"application/json; charset=utf-8"}

func (j JSON) Render(w http.ResponseWriter, status int) error {
	w.WriteHeader(status)
	return WriteJSON(w, j.Data)
}
func (j JSON) WriteContentType(w http.ResponseWriter) {
	writeContentType(w, jsonContentType[0])
}

func WriteJSON(w http.ResponseWriter, obj any) error {
	writeContentType(w, jsonContentType[0])
	jsonBytes, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	_, err = w.Write(jsonBytes)
	return err
}
