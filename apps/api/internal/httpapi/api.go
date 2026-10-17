package httpapi

import (
	"context"
	"log/slog"
	"net/http"

	"antares/api/internal/config"
	"antares/api/internal/service"
	"antares/api/internal/store"
	"antares/api/internal/upstream"
)

type Backend interface {
	Resolve(ctx context.Context, in service.ResolveInput) (store.Target, error)
	Extract(ctx context.Context, in service.ExtractInput) (service.Outcome, error)
	Analyze(ctx context.Context, in service.ExtractInput) (service.Outcome, error)
	Predict(ctx context.Context, in service.PredictInput) (service.PredictResult, error)
	Runs(ctx context.Context) ([]upstream.Run, error)
	Watch(ctx context.Context, token string, since uint64) (*service.Watch, error)
	Status(ctx context.Context, token string) (service.SubscriptionView, error)
	Versions() (shaula, fang string)
}

type API struct {
	be  Backend
	cfg config.Config
	log *slog.Logger
}

type Route struct {
	Method  string
	Pattern string
}

type route struct {
	Route
	handle func(*API, http.ResponseWriter, *http.Request)
}

var routes = []route{
	{Route{"GET", "/healthz"}, (*API).health},
	{Route{"POST", "/targets/resolve"}, (*API).resolve},
	{Route{"GET", "/ml/runs"}, (*API).runs},
	{Route{"POST", "/ml/predict"}, (*API).predict},
	{Route{"POST", "/pipeline/extract"}, (*API).extract},
	{Route{"POST", "/analyze"}, (*API).analyze},
	{Route{"GET", "/subscription/{token}"}, (*API).subscription},
	{Route{"GET", "/openapi.yaml"}, (*API).spec},
	{Route{"GET", "/docs"}, (*API).docs},
}

// Routes lists every registered method and pattern.
func Routes() []Route {
	out := make([]Route, len(routes))
	for i, r := range routes {
		out[i] = r.Route
	}
	return out
}

func New(be Backend, cfg config.Config, log *slog.Logger) http.Handler {
	a := &API{be: be, cfg: cfg, log: log}
	mux := http.NewServeMux()
	for _, r := range routes {
		mux.HandleFunc(r.Method+" "+r.Pattern, func(w http.ResponseWriter, req *http.Request) { r.handle(a, w, req) })
	}
	return a.recoverer(a.cors(a.logging(mux)))
}
