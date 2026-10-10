package httpapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestInferenceResponseSurvivesOrdinaryServerWriteTimeout(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		request, cancel := inferenceRequest(w, request)
		defer cancel()
		timer := time.NewTimer(40 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-timer.C:
			_, _ = io.WriteString(w, "completed")
		case <-request.Context().Done():
		}
	}))
	server.Config.WriteTimeout = 20 * time.Millisecond
	server.Start()
	defer server.Close()
	response, err := server.Client().Get(server.URL)
	require.NoError(t, err)
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, "completed", string(body))
}
