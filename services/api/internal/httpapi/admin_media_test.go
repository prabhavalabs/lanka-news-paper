package httpapi

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateSourceLogo(t *testing.T) {
	var valid bytes.Buffer
	tooSmall := image.NewRGBA(image.Rect(0, 0, 32, 32))
	tooSmall.Set(0, 0, color.Black)
	require.NoError(t, png.Encode(&valid, tooSmall))
	_, _, err := validateSourceLogo(valid.Bytes())
	require.ErrorContains(t, err, "at least 64")

	valid.Reset()
	require.NoError(t, png.Encode(&valid, image.NewRGBA(image.Rect(0, 0, 128, 128))))
	contentType, extension, err := validateSourceLogo(valid.Bytes())
	require.NoError(t, err)
	require.Equal(t, "image/png", contentType)
	require.Equal(t, "png", extension)

	_, _, err = validateSourceLogo([]byte("not an image"))
	require.ErrorContains(t, err, "only PNG and JPEG")
}
