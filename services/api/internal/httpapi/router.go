package httpapi

import (
	"net/http"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/publish"
)

type Dependencies struct {
	AllowedOrigins []string
	Database       DatabaseChecker
	News           publish.Reader
}

func NewRouter(dependencies Dependencies) http.Handler {
	mux := http.NewServeMux()
	health := healthHandler{database: dependencies.Database}
	news := newsHandler{reader: dependencies.News}

	mux.HandleFunc("GET /api/v1/health/live", health.liveness)
	mux.HandleFunc("GET /api/v1/health/ready", health.readiness)
	mux.HandleFunc("GET /api/v1/news", news.list)

	return withRecovery(withRequestID(withSecurityHeaders(withCORS(mux, dependencies.AllowedOrigins))))
}
