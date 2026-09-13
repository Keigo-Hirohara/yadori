package sharedhttp

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

const (
	CodeInvalidRequestBody = "INVALID_REQUEST_BODY"
	CodeInternal           = "INTERNAL_ERROR"
	CodeTimeout            = "REQUEST_TIMEOUT"
	CodeNotFound           = "NOT_FOUND"
)

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("レスポンスの書き込みに失敗しました", "error", err)
	}
}

func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, ErrorResponse{Code: code, Message: message})
}

func WriteNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func WriteBadRequest(w http.ResponseWriter) {
	WriteError(w, http.StatusBadRequest, CodeInvalidRequestBody, "リクエストの形式が不正です")
}

func WriteInternal(w http.ResponseWriter, err error) {
	slog.Error("想定外のエラー", "error", err)
	WriteError(w, http.StatusInternalServerError, CodeInternal, "サーバーエラーが発生しました")
}
