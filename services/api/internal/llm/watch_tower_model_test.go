package llm

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestWatchTowerAssignmentRejectsInvalidProviderModelAndEffort(t *testing.T) {
	gateway := NewGateway(nil)
	for _, test := range []struct{ provider, model, effort string }{
		{"retired", "gpt-6-luna", "low"},
		{"codex_cli", "unknown", "low"},
		{"codex_cli", "gpt-6-luna", "extreme"},
	} {
		err := gateway.UpdateWatchTowerModel(context.Background(), uuid.Nil, test.provider, test.model, test.effort)
		require.ErrorIs(t, err, ErrWatchTowerModelInvalid)
	}
}
