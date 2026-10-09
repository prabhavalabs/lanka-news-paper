package llm

import "context"

// Completer is the boundary used by editorial workflows. Complete resolves the
// task profile; CompleteWithModel pins administrative work to a selected model.
type Completer interface {
	Complete(context.Context, Request) (Response, error)
	CompleteWithModel(context.Context, Request, string, string) (Response, error)
}
