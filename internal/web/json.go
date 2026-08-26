package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"quarantine-workbench/internal/domain"
)

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, domain.FieldError("body", "请求 JSON 无效："+err.Error()))
		return false
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		writeError(w, domain.FieldError("body", "请求只能包含一个 JSON 对象"))
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	var e *domain.Error
	if !errors.As(err, &e) {
		writeJSON(w, 500, map[string]any{"error": map[string]string{"code": "internal_error", "message": "服务内部错误"}})
		return
	}
	status := 400
	switch e.Code {
	case domain.CodeNotFound:
		status = 404
	case domain.CodeConflict:
		status = 409
	case domain.CodeDuplicate:
		status = 409
	case domain.CodeForbidden:
		status = 403
	case domain.CodeState:
		status = 422
	}
	writeJSON(w, status, map[string]any{"error": e})
}
