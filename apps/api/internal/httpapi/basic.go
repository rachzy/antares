package httpapi

import (
	"net/http"

	"antares/api/internal/service"
	"antares/api/internal/upstream"
)

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	shaula, fang := a.be.Versions()
	orUnknown := func(v string) string {
		if v == "" {
			return "unknown"
		}
		return v
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":         "ok",
		"ready":          shaula != "" && fang != "",
		"shaula_version": orUnknown(shaula),
		"fang_version":   orUnknown(fang),
	})
}

type resolveRequest struct {
	Query   string   `json:"query"`
	Mission string   `json:"mission"`
	RA      *float64 `json:"ra_deg"`
	Dec     *float64 `json:"dec_deg"`
}

func (a *API) resolve(w http.ResponseWriter, r *http.Request) {
	var req resolveRequest
	if err := decode(w, r, &req); err != nil {
		a.writeError(w, r, err)
		return
	}
	target, err := a.be.Resolve(r.Context(), service.ResolveInput{
		Query: req.Query, Mission: req.Mission, RA: req.RA, Dec: req.Dec,
	})
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, service.InfoOf(target))
}

func (a *API) runs(w http.ResponseWriter, r *http.Request) {
	runs, err := a.be.Runs(r.Context())
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	if runs == nil {
		runs = []upstream.Run{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": runs})
}

func (a *API) runHyperparameters(w http.ResponseWriter, r *http.Request) {
	settings, err := a.be.RunSettings(r.Context(), r.PathValue("run_id"))
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (a *API) runDataset(w http.ResponseWriter, r *http.Request) {
	dataset, err := a.be.RunDataset(r.Context(), r.PathValue("run_id"))
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, dataset)
}

type predictRequest struct {
	Features []map[string]any `json:"features"`
	RunID    string           `json:"run_id"`
	StarID   string           `json:"star_id"`
}

func (a *API) predict(w http.ResponseWriter, r *http.Request) {
	var req predictRequest
	if err := decode(w, r, &req); err != nil {
		a.writeError(w, r, err)
		return
	}
	result, err := a.be.Predict(r.Context(), service.PredictInput{
		Rows: req.Features, RunID: req.RunID, StarID: req.StarID,
	})
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
