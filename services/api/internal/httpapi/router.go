package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/desk"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/iam"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/ingest"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/media"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/publish"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/registry"
)

type Dependencies struct {
	AllowedOrigins []string
	CookieSecure   bool
	Database       DatabaseChecker
	Desk           *desk.Store
	IAM            *iam.Store
	LLM            *llm.Gateway
	Media          *media.Store
	News           *publish.Store
	Poller         *ingest.Poller
	Registry       *registry.Store
	RunPipeline    func(context.Context, string, string) error
	SessionTTL     time.Duration
}

func NewRouter(dependencies Dependencies) http.Handler {
	mux := http.NewServeMux()
	health := healthHandler{database: dependencies.Database}
	news := newsHandler{reader: dependencies.News}
	auth := newAuthHandler(dependencies.IAM, dependencies.SessionTTL, dependencies.CookieSecure)
	admin := adminHandler{registry: dependencies.Registry, poller: dependencies.Poller, llm: dependencies.LLM, desk: dependencies.Desk, media: dependencies.Media, runPipeline: dependencies.RunPipeline}

	mux.HandleFunc("GET /api/v1/health/live", health.liveness)
	mux.HandleFunc("GET /api/v1/health/ready", health.readiness)
	mux.HandleFunc("GET /api/v1/news", news.list)
	mux.HandleFunc("GET /api/v1/news/{id}", news.one)
	mux.HandleFunc("GET /api/v1/categories", news.categories)
	mux.HandleFunc("GET /api/v1/sources", news.sources)
	mux.HandleFunc("GET /api/v1/sources/{id}", news.source)
	mux.HandleFunc("GET /api/v1/search", news.list)
	mux.HandleFunc("GET /api/v1/events", news.events)
	mux.HandleFunc("GET /api/v1/events/{id}", news.event)
	mux.HandleFunc("GET /api/v1/breaking", news.breaking)
	mux.HandleFunc("GET /api/v1/brief", news.brief)
	mux.HandleFunc("POST /api/v1/complaints", news.complain)
	mux.Handle("POST /api/admin/login", withCSRF(http.HandlerFunc(auth.login)))
	mux.Handle("POST /api/admin/logout", withCSRF(http.HandlerFunc(auth.logout)))

	protected := http.NewServeMux()
	protected.HandleFunc("GET /api/admin/me", auth.me)
	protected.HandleFunc("GET /api/admin/sources", admin.sources)
	protected.HandleFunc("POST /api/admin/sources", admin.sources)
	protected.HandleFunc("GET /api/admin/sources/{id}", admin.source)
	protected.HandleFunc("GET /api/admin/sources/{id}/performance", admin.sourcePerformance)
	protected.HandleFunc("POST /api/admin/sources/{id}/logo", admin.sourceLogo)
	protected.HandleFunc("DELETE /api/admin/sources/{id}/logo", admin.sourceLogo)
	protected.HandleFunc("POST /api/admin/sources/{id}/active", admin.setActive)
	protected.HandleFunc("GET /api/admin/sources/{id}/endpoints", admin.endpoints)
	protected.HandleFunc("POST /api/admin/sources/{id}/endpoints", admin.endpoints)
	protected.HandleFunc("GET /api/admin/sources/{id}/rights", admin.rights)
	protected.HandleFunc("POST /api/admin/sources/{id}/rights", admin.rights)
	protected.HandleFunc("POST /api/admin/endpoints/{endpointId}/pause", admin.pause)
	protected.HandleFunc("POST /api/admin/endpoints/{endpointId}/test", admin.test)
	protected.HandleFunc("POST /api/admin/endpoints/{endpointId}/run", admin.runNow)
	protected.HandleFunc("GET /api/admin/overview", admin.overview)
	protected.HandleFunc("GET /api/admin/overview/trends", admin.trends)
	protected.HandleFunc("GET /api/admin/knowledge-graph", admin.knowledgeGraph)
	protected.HandleFunc("GET /api/admin/queue", admin.queue)
	protected.HandleFunc("GET /api/admin/jobs", admin.jobs)
	protected.HandleFunc("GET /api/admin/articles", admin.articles)
	protected.HandleFunc("GET /api/admin/articles/{id}", admin.article)
	protected.HandleFunc("POST /api/admin/articles/{id}/pipeline/run", admin.runArticlePipeline)
	protected.HandleFunc("GET /api/admin/quarantine", admin.quarantine)
	protected.HandleFunc("POST /api/admin/articles/{id}/status", admin.articleStatus)
	protected.HandleFunc("POST /api/admin/articles/{id}/category", admin.articleCategory)
	protected.HandleFunc("POST /api/admin/articles/{id}/note", admin.articleNote)
	protected.HandleFunc("GET /api/admin/complaints", admin.complaints)
	protected.HandleFunc("POST /api/admin/complaints/{id}", admin.resolveComplaint)
	protected.HandleFunc("POST /api/admin/categories/{slug}/hold", admin.holdCategory)
	protected.HandleFunc("POST /api/admin/events/{id}/lock", admin.lockEvent)
	protected.HandleFunc("POST /api/admin/sources/{id}/archive", admin.archive)
	protected.HandleFunc("POST /api/admin/sources/{id}", admin.updateSource)
	protected.HandleFunc("POST /api/admin/endpoints/{endpointId}", admin.updateEndpoint)
	protected.HandleFunc("GET /api/admin/llm/providers", admin.providers)
	protected.HandleFunc("GET /api/admin/llm/models", admin.models)
	protected.HandleFunc("GET /api/admin/llm/profiles", admin.profiles)
	protected.HandleFunc("POST /api/admin/llm/profiles", admin.profiles)
	protected.HandleFunc("GET /api/admin/media/{key...}", admin.mediaFile)
	mux.Handle("/api/admin/", withCSRF(auth.requireAuth(protected)))

	return withRecovery(withRequestID(withSecurityHeaders(withCORS(http.MaxBytesHandler(mux, 1<<20), dependencies.AllowedOrigins))))
}
