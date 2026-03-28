package handlers

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strconv"

	"github.com/julienschmidt/httprouter"
)

func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		_, _ = w.Write([]byte("null\n"))
		return
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice:
		if rv.IsNil() {
			_, _ = w.Write([]byte("[]\n"))
			return
		}
	case reflect.Map:
		if rv.IsNil() {
			_, _ = w.Write([]byte("{}\n"))
			return
		}
	}
	_ = json.NewEncoder(w).Encode(v)
}

func WriteError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, map[string]string{"error": msg})
}

func ParseID(ps httprouter.Params, name string) (int64, error) {
	return strconv.ParseInt(ps.ByName(name), 10, 64)
}

func ReadJSON(r *http.Request, v interface{}) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
