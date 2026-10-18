package httpapi

import (
	"math"
	"net/http"
	"strconv"

	"antares/api/internal/jobs"
	"antares/api/internal/service"
)

// latestFeatures answers with the newest cached features of the star a name or position identifies.
func (a *API) latestFeatures(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	in := service.ResolveInput{Query: q.Get("query"), Mission: q.Get("mission")}
	var err error
	if in.RA, err = coordinate(q.Get("ra_deg"), "ra_deg"); err != nil {
		a.writeError(w, r, err)
		return
	}
	if in.Dec, err = coordinate(q.Get("dec_deg"), "dec_deg"); err != nil {
		a.writeError(w, r, err)
		return
	}
	if (in.RA == nil) != (in.Dec == nil) {
		a.writeError(w, r, badRequest("ra_deg and dec_deg must be sent together"))
		return
	}
	view, err := a.be.LatestFeatures(r.Context(), in)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// subscriptionFeatures answers with the newest cached features of a job's star. A malformed token
// is rejected before any database lookup.
func (a *API) subscriptionFeatures(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if !jobs.ValidToken(token) {
		a.writeError(w, r, &service.Error{Kind: service.KindNotFound, Message: "unknown or expired subscription"})
		return
	}
	view, err := a.be.LatestFeaturesByToken(r.Context(), token)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// coordinate parses an optional finite float query parameter; nil means it was not sent.
func coordinate(raw, name string) (*float64, error) {
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return nil, badRequest("%s must be a number", name)
	}
	return &v, nil
}
