package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const maxJSONBodyBytes int64 = 1 << 20

func decodeJSON(request *http.Request, dest any) error {
	defer request.Body.Close()
	limitedBody := &io.LimitedReader{R: request.Body, N: maxJSONBodyBytes + 1}
	decoder := json.NewDecoder(limitedBody)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return err
	}
	if limitedBody.N == 0 {
		return errors.New("JSON body exceeds 1 MiB")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("JSON body must contain exactly one document")
		}
		return err
	}
	if limitedBody.N == 0 {
		return errors.New("JSON body exceeds 1 MiB")
	}
	return nil
}
