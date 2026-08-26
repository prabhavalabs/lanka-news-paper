package llm

import (
	"context"
	"strings"
)

// Completer is the provider-neutral boundary used by editorial workflows.
// Complete resolves the configured task profile, while CompleteWithModel pins
// a run to one explicitly selected provider and model.
type Completer interface {
	Complete(context.Context, Request) (Response, error)
	CompleteWithModel(context.Context, Request, string, string) (Response, error)
}

// ModelProvider adapts an independently implemented provider to the router.
// Providers own their transport, authentication and audit logging.
type ModelProvider interface {
	ID() string
	Complete(context.Context, Request, string) (Response, error)
}

type Router struct {
	defaults  Completer
	providers map[string]ModelProvider
}

func NewRouter(defaults Completer, providers ...ModelProvider) *Router {
	router := &Router{defaults: defaults, providers: make(map[string]ModelProvider, len(providers))}
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		if id := strings.TrimSpace(provider.ID()); id != "" {
			router.providers[id] = provider
		}
	}
	return router
}

func (router *Router) Complete(ctx context.Context, request Request) (Response, error) {
	return router.defaults.Complete(ctx, request)
}

func (router *Router) CompleteWithModel(ctx context.Context, request Request, providerID, model string) (Response, error) {
	providerID = strings.TrimSpace(providerID)
	if provider, ok := router.providers[providerID]; ok {
		return provider.Complete(ctx, request, strings.TrimSpace(model))
	}
	return router.defaults.CompleteWithModel(ctx, request, providerID, model)
}
