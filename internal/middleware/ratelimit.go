package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/httprate"
)

func PerIP(requests int, window time.Duration) func(http.Handler) http.Handler {
	return httprate.LimitByIP(requests, window)
}
