package binding

import (
	"encoding/xml"
	"net/http"
)

type xmlBinding struct {
	DisallowUnknownFields bool
	IsValidate            bool
}

func (x xmlBinding) Name() string {
	return "xml"
}

func (x xmlBinding) Bind(r *http.Request, obj any) error {
	if r.Body == nil {
		return nil
	}
	decoder := xml.NewDecoder(r.Body)
	if err := decoder.Decode(obj); err != nil {
		return err
	}
	return validate(obj)
}
