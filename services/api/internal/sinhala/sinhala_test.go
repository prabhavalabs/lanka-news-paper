package sinhala

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPredominantDetectsSinhala(t *testing.T) {
	require.True(t, Predominant("ශ්‍රී ලංකා රජය අද නිවේදනයක් නිකුත් කළේය"))
}

func TestPredominantRejectsEnglish(t *testing.T) {
	require.False(t, Predominant("The government issued a statement today in Colombo"))
}
