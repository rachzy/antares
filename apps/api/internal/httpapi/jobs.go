package httpapi

import (
	"context"
	"net/http"

	"github.com/rachzy/antares/apps/api/internal/jobs"
	"github.com/rachzy/antares/apps/api/internal/service"
)

type extractRequest struct {
	Query        string   `json:"query"`
	RA           *float64 `json:"ra_deg"`
	Dec          *float64 `json:"dec_deg"`
	Mission      string   `json:"mission"`
	SigmaClip    *float64 `json:"sigma_clip"`
	DownloadAll  bool     `json:"download_all"`
	Author       string   `json:"author"`
	Exptime      string   `json:"exptime"`
	UseTLS       bool     `json:"use_tls"`
	MaskEclipses bool     `json:"mask_eclipses"`
	ForceRefresh bool     `json:"force_refresh"`
	RunID        string   `json:"run_id"`
	StarID       string   `json:"star_id"`
}

func (r extractRequest) input() service.ExtractInput {
	return service.ExtractInput{
		Query: r.Query, RA: r.RA, Dec: r.Dec, Mission: r.Mission, SigmaClip: r.SigmaClip,
		DownloadAll: r.DownloadAll, Author: r.Author, Exptime: r.Exptime, UseTLS: r.UseTLS,
		MaskEclipses: r.MaskEclipses, ForceRefresh: r.ForceRefresh, RunID: r.RunID, StarID: r.StarID,
	}
}

type doneBody struct {
	Status string       `json:"status"`
	Result *jobs.Result `json:"result"`
}

type queuedBody struct {
	Status           string `json:"status"`
	Token            string `json:"token"`
	SubscriptionPath string `json:"subscription_path"`
	Position         int    `json:"position"`
}

func (a *API) extract(w http.ResponseWriter, r *http.Request) {
	a.submit(w, r, a.be.Extract)
}

func (a *API) analyze(w http.ResponseWriter, r *http.Request) {
	a.submit(w, r, a.be.Analyze)
}

func (a *API) submit(w http.ResponseWriter, r *http.Request, call func(context.Context, service.ExtractInput) (service.Outcome, error)) {
	var req extractRequest
	if err := decode(w, r, &req); err != nil {
		a.writeError(w, r, err)
		return
	}
	outcome, err := call(r.Context(), req.input())
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	if outcome.Result != nil {
		writeJSON(w, http.StatusOK, doneBody{Status: "done", Result: outcome.Result})
		return
	}
	status := "queued"
	if outcome.Position == 0 {
		status = "running"
	}
	writeJSON(w, http.StatusAccepted, queuedBody{
		Status:           status,
		Token:            outcome.Token,
		SubscriptionPath: "/subscription/" + outcome.Token,
		Position:         outcome.Position,
	})
}
