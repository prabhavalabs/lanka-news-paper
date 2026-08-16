package llm

import "context"

type Request struct {
	Task   string
	System string
	Input  string
}

type Response struct {
	Text string
}

type Gateway interface {
	Complete(ctx context.Context, request Request) (Response, error)
}
