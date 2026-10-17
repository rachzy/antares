package httpapi

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"slices"
	"time"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(status int) {
	s.status = status
	s.ResponseWriter.WriteHeader(status)
}

func (s *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := s.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("hijacking is not supported")
	}
	return hijacker.Hijack()
}

func (s *statusWriter) Flush() {
	if flusher, ok := s.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (s *statusWriter) Unwrap() http.ResponseWriter { return s.ResponseWriter }

func (a *API) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				a.log.Error("handler panicked", "route", r.Pattern, "panic", recovered)
				a.writeError(w, r, errors.New("panic"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (a *API) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && slices.Contains(a.cfg.AllowedOrigins, origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// The subscription token is part of the URL path, so only the route pattern is logged.
func (a *API) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		a.log.Info("request", "method", r.Method, "route", r.Pattern, "status", sw.status, "duration", time.Since(start))
	})
}
