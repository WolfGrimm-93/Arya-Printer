package apiserver

import (
	"encoding/json"
	"net/http"

	"aryaescpos/internal/contract"
)

// decodeJSON decodes r.Body into v, returning a *contract.APIError (400)
// on malformed JSON instead of a raw decode error, so writeError renders a
// consistent {"detail": ...} body.
func decodeJSON(r *http.Request, v any) *contract.APIError {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return contract.BadRequest("invalid_json", "request body is not valid JSON: "+err.Error())
	}
	return nil
}

// writeJSON encodes v as the JSON response body with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError translates err into an HTTP response. A *contract.APIError is
// rendered as {"detail": message} with its own HTTPStatus, matching every
// handler's error shape. Any other error is never shown to the client —
// only a generic 500 — so internal error strings (file paths, driver
// errors, stack fragments) never leak over the wire.
func writeError(w http.ResponseWriter, err error) {
	if apiErr, ok := err.(*contract.APIError); ok {
		writeJSON(w, apiErr.HTTPStatus, map[string]string{"detail": apiErr.Message})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "internal server error"})
}
