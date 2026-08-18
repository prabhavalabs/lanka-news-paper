package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
)

type requestIDContextKey struct{}

func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		w.Header().Set("X-Request-ID", requestID)
		request = request.WithContext(context.WithValue(request.Context(), requestIDContextKey{}, requestID))
		next.ServeHTTP(w, request)
	})
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, request)
	})
}

func withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.ErrorContext(request.Context(), "request panic recovered", "panic", recovered)
				writeProblem(
					w,
					http.StatusInternalServerError,
					"https://snap.local/problems/internal",
					"Internal server error",
					"The request could not be completed.",
				)
			}
		}()
		next.ServeHTTP(w, request)
	})
}

func withCORS(next http.Handler, origins []string) http.Handler {
	allowed := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		allowed[origin] = struct{}{}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		origin := request.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, request)
			return
		}

		w.Header().Add("Vary", "Origin")
		if _, ok := allowed[origin]; !ok {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-SNAP-CSRF")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Origin", origin)
		if request.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, request)
	})
}

func withCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead && request.Method != http.MethodOptions && request.Header.Get("X-SNAP-CSRF") != "1" {
			writeProblem(w, http.StatusForbidden, "https://snap.local/problems/csrf", "Forbidden", "Request verification failed.")
			return
		}
		next.ServeHTTP(w, request)
	})
}

func newRequestID() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "request-id-unavailable"
	}
	return hex.EncodeToString(buffer)
}
