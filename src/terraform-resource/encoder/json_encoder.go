package encoder

import (
	"encoding/json"
	"io"
)

func NewJSONEncoder(w io.Writer) *json.Encoder {
	e := json.NewEncoder(w)
	e.SetEscapeHTML(false)
	return e
}
