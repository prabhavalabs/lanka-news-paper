package classify

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFromDetectsSport(t *testing.T) {
	slug, confidence := From(nil, "ශ්‍රී ලංකා ක්‍රිකට් කණ්ඩායම ජයග්‍රහණය කළේය")
	require.Equal(t, "sport", slug)
	require.Greater(t, confidence, 0.5)
}
