package httpapi

import (
	"encoding/json"
	"net/http"
)

func decodeJSON(request *http.Request, dest any) error {
	defer request.Body.Close()
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(dest)
}
