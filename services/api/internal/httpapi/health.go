package httpapi

import (
	"context"
	"net/http"
)

type DatabaseChecker interface {
	Ping(context.Context) error
}

type healthHandler struct {
	database DatabaseChecker
}

func (handler healthHandler) liveness(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"service": "snap-api",
		"status":  "ok",
	})
}

func (handler healthHandler) readiness(w http.ResponseWriter, request *http.Request) {
	if handler.database == nil || handler.database.Ping(request.Context()) != nil {
		writeProblem(
			w,
			http.StatusServiceUnavailable,
			"https://snap.local/problems/dependency-unavailable",
			"Service unavailable",
			"A required dependency is unavailable.",
		)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"checks": map[string]string{"database": "ok"},
		"status": "ready",
	})
}
