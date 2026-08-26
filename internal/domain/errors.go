package domain

import "fmt"

type ErrorCode string

const (
	CodeValidation ErrorCode = "validation_error"
	CodeConflict   ErrorCode = "revision_conflict"
	CodeNotFound   ErrorCode = "not_found"
	CodeForbidden  ErrorCode = "forbidden"
	CodeState      ErrorCode = "invalid_state"
	CodeDuplicate  ErrorCode = "duplicate_accession"
)

type Error struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Field   string    `json:"field,omitempty"`
	Details any       `json:"details,omitempty"`
}

func (e *Error) Error() string { return e.Message }

func NewError(code ErrorCode, message string) *Error {
	return &Error{Code: code, Message: message}
}

func FieldError(field, message string) *Error {
	return &Error{Code: CodeValidation, Field: field, Message: message}
}

func StateError(status CaseStatus, action string) *Error {
	return &Error{Code: CodeState, Message: fmt.Sprintf("个案状态 %s 不允许%s", status, action)}
}
