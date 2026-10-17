package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/rachzy/antares/apps/api/internal/service"
)

const maxBodyBytes = 1 << 20

type httpError struct {
	Status  int
	Code    string
	Message string
}

func (e *httpError) Error() string { return e.Message }

func badRequest(format string, args ...any) *httpError {
	return &httpError{Status: http.StatusBadRequest, Code: "invalid_request", Message: fmt.Sprintf(format, args...)}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func decode(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return &httpError{Status: http.StatusRequestEntityTooLarge, Code: "too_large", Message: "request body is larger than 1 MiB"}
		}
		if errors.Is(err, io.EOF) {
			return badRequest("request body is empty")
		}
		return badRequest("invalid JSON body: %v", err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return badRequest("unexpected data after the JSON body")
	}
	return nil
}

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (a *API) writeError(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message := http.StatusInternalServerError, "internal", "internal error"

	var he *httpError
	var se *service.Error
	switch {
	case errors.As(err, &he):
		status, code, message = he.Status, he.Code, he.Message
	case errors.As(err, &se):
		switch se.Kind {
		case service.KindInvalid:
			status, code, message = http.StatusBadRequest, "invalid_request", se.Message
		case service.KindNotFound:
			status, code, message = http.StatusNotFound, "not_found", se.Message
		case service.KindUnprocessable:
			status, code, message = http.StatusUnprocessableEntity, "unprocessable", se.Message
		case service.KindUnavailable:
			status, code, message = http.StatusServiceUnavailable, "unavailable", se.Message
		case service.KindBusy:
			w.Header().Set("Retry-After", "30")
			status, code, message = http.StatusServiceUnavailable, "busy", se.Message
		default:
			a.log.Error("request failed", "route", r.Pattern, "error", se.Message)
		}
	default:
		a.log.Error("request failed", "route", r.Pattern, "error", err)
	}

	var body errorBody
	body.Error.Code, body.Error.Message = code, message
	writeJSON(w, status, body)
}
