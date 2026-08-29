package httpapi

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodeJSONAcceptsOneValidDocument(t *testing.T) {
	request := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"reader"}`))
	var body struct {
		Name string `json:"name"`
	}

	err := decodeJSON(request, &body)

	require.NoError(t, err)
	require.Equal(t, "reader", body.Name)
}

func TestDecodeJSONRejectsUnknownFields(t *testing.T) {
	request := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"reader","unexpected":true}`))
	var body struct {
		Name string `json:"name"`
	}

	err := decodeJSON(request, &body)

	require.Error(t, err)
}

func TestDecodeJSONRejectsTrailingDocument(t *testing.T) {
	request := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"reader"} {"name":"second"}`))
	var body struct {
		Name string `json:"name"`
	}

	err := decodeJSON(request, &body)

	require.Error(t, err)
}

func TestDecodeJSONRejectsOversizedBody(t *testing.T) {
	body := `{"name":"` + strings.Repeat("a", int(maxJSONBodyBytes)) + `"}`
	request := httptest.NewRequest("POST", "/", strings.NewReader(body))
	var decoded struct {
		Name string `json:"name"`
	}

	err := decodeJSON(request, &decoded)

	require.Error(t, err)
}
