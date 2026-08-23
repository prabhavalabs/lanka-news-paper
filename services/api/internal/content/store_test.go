package content

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEffectiveCrawlDelaySeconds(t *testing.T) {
	t.Parallel()

	require.Equal(t, 5, effectiveCrawlDelaySeconds(5, 0))
	require.Equal(t, 5, effectiveCrawlDelaySeconds(5, 2*time.Second))
	require.Equal(t, 4, effectiveCrawlDelaySeconds(1, 3500*time.Millisecond))
	require.Equal(t, 3600, effectiveCrawlDelaySeconds(5, time.Hour))
}
