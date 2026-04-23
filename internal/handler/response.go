package handler

import (
	"net/http"

	"github.com/mossandoval/datil-api/internal/httpx"
)

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	httpx.WriteJSON(w, status, payload)
}

func WriteError(w http.ResponseWriter, status int, message string, fields map[string]string) {
	httpx.WriteError(w, status, message, fields)
}

func WriteNoContent(w http.ResponseWriter) {
	httpx.WriteNoContent(w)
}

func ReadJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	return httpx.ReadJSON(w, r, dst)
}
