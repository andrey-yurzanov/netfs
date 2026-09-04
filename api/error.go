package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type ResponseErrorCode int

func (code ResponseErrorCode) Is(err error) bool {
	resError, ok := err.(*ResponseError)
	if ok {
		return resError.Code == code
	}
	return false
}

func (code ResponseErrorCode) HttpStatusCode() int {
	if code == ErrParameterIsRequired {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

const (
	ErrFileAlreadyExists ResponseErrorCode = 1 << iota
	ErrFileNotFound

	ErrParameterIsRequired
)

type ResponseError struct {
	Message string
	Code    ResponseErrorCode
}

func (err *ResponseError) Error() string {
	return err.Message
}

func (err *ResponseError) Is(target error) bool {
	resError, ok := target.(*ResponseError)
	if ok {
		return resError.Code == err.Code
	}
	return false
}

func NewResponseError(code ResponseErrorCode, messages ...string) *ResponseError {
	return &ResponseError{Code: code, Message: strings.Join(messages, "")}
}

func unmarshalError(reader io.Reader) error {
	data, err := io.ReadAll(reader)
	if err == nil {
		resErr := &ResponseError{}
		if err = json.Unmarshal(data, resErr); err == nil {
			err = resErr
		}
	}
	return err
}
