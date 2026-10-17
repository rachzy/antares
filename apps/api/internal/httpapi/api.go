package httpapi

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/rachzy/antares/apps/api/internal/config"
	"github.com/rachzy/antares/apps/api/internal/service"
	"github.com/rachzy/antares/apps/api/internal/store"
	"github.com/rachzy/antares/apps/api/internal/upstream"
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

func New(be Backend, cfg config.Config, log *slog.Logger) http.Handler {
	a := &API{be: be, cfg: cfg, log: log}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.health)
	mux.HandleFunc("POST /targets/resolve", a.resolve)
	mux.HandleFunc("GET /ml/runs", a.runs)
	mux.HandleFunc("POST /ml/predict", a.predict)
	mux.HandleFunc("POST /pipeline/extract", a.extract)
	mux.HandleFunc("POST /analyze", a.analyze)
	return a.recoverer(a.cors(a.logging(mux)))
}
