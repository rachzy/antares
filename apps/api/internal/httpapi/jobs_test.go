package httpapi_test

import (
	"context"
	"encoding/json"
	"testing"

	"antares/api/internal/jobs"
	"antares/api/internal/service"
)

func cachedOutcome() service.Outcome {
	return service.Outcome{Result: &jobs.Result{
		Target:        jobs.TargetInfo{Catalog: "KIC", CatalogID: "8120608", Designation: "KIC 8120608"},
		Cached:        true,
		ShaulaVersion: "0.0.1a0",
		Features:      []map[string]any{{"period_days": 129.9, "vshape_metric": nil}},
	}}
}

func TestACachedExtractionIsAnImmediate200(t *testing.T) {
	h := handler(t, &stub{extract: func(context.Context, service.ExtractInput) (service.Outcome, error) { return cachedOutcome(), nil }}, nil)
	rec := do(h, "POST", "/pipeline/extract", `{"query":"Kepler-186"}`)
	if rec.Code != 200 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Status string         `json:"status"`
		Result map[string]any `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "done" || body.Result["cached"] != true || body.Result["shaula_version"] != "0.0.1a0" {
		t.Fatalf("body = %+v", body)
	}
	features := body.Result["features"].([]any)
	if features[0].(map[string]any)["vshape_metric"] != nil {
		t.Fatalf("a missing value must be null on the wire: %v", features)
	}
}

func TestAQueuedExtractionIs202WithTheSubscriptionPath(t *testing.T) {
	token := "T0KEN0123456789012345678901234567890123456"
	h := handler(t, &stub{extract: func(context.Context, service.ExtractInput) (service.Outcome, error) {
		return service.Outcome{Token: token, Position: 2}, nil
	}}, nil)

	rec := do(h, "POST", "/pipeline/extract", `{"query":"Kepler-186"}`)
	if rec.Code != 202 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["status"] != "queued" || body["token"] != token || body["subscription_path"] != "/subscription/"+token || body["position"] != float64(2) {
		t.Fatalf("body = %v", body)
	}

	running := handler(t, &stub{extract: func(context.Context, service.ExtractInput) (service.Outcome, error) {
		return service.Outcome{Token: token, Position: 0}, nil
	}}, nil)
	body = nil
	_ = json.Unmarshal(do(running, "POST", "/pipeline/extract", `{"query":"x"}`).Body.Bytes(), &body)
	if body["status"] != "running" {
		t.Fatalf("position 0 means the job is already running, got status %v", body["status"])
	}
}

func TestEveryExtractFieldReachesTheService(t *testing.T) {
	var got service.ExtractInput
	h := handler(t, &stub{extract: func(_ context.Context, in service.ExtractInput) (service.Outcome, error) {
		got = in
		return cachedOutcome(), nil
	}}, nil)

	rec := do(h, "POST", "/pipeline/extract", `{
		"query":"Kepler-186f","mission":"TESS","sigma_clip":3.5,
		"download_all":true,"author":"SPOC","exptime":"short","use_tls":true,"mask_eclipses":true,
		"force_refresh":true,"run_id":"r1","star_id":"mine"}`)
	if rec.Code != 200 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if got.Query != "Kepler-186f" || got.Mission != "TESS" || got.SigmaClip == nil || *got.SigmaClip != 3.5 ||
		!got.DownloadAll || got.Author != "SPOC" || got.Exptime != "short" || !got.UseTLS || !got.MaskEclipses ||
		!got.ForceRefresh || got.RunID != "r1" || got.StarID != "mine" {
		t.Fatalf("service input = %+v", got)
	}
}

func TestExtractForwardsCoordinatesInsteadOfAQuery(t *testing.T) {
	var got service.ExtractInput
	h := handler(t, &stub{extract: func(_ context.Context, in service.ExtractInput) (service.Outcome, error) {
		got = in
		return cachedOutcome(), nil
	}}, nil)

	rec := do(h, "POST", "/pipeline/extract", `{"ra_deg":298.65,"dec_deg":43.95}`)
	if rec.Code != 200 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if got.Query != "" || got.RA == nil || *got.RA != 298.65 || got.Dec == nil || *got.Dec != 43.95 {
		t.Fatalf("service input = %+v", got)
	}
}

func TestOmittedOptionalFieldsStayUnset(t *testing.T) {
	var got service.ExtractInput
	called := 0
	h := handler(t, &stub{extract: func(_ context.Context, in service.ExtractInput) (service.Outcome, error) {
		got = in
		called++
		return cachedOutcome(), nil
	}}, nil)
	rec := do(h, "POST", "/pipeline/extract", `{"query":"Kepler-186"}`)
	if rec.Code != 200 || called != 1 {
		t.Fatalf("status %d, backend calls %d, body %s", rec.Code, called, rec.Body.String())
	}
	if got.SigmaClip != nil || got.RA != nil || got.Dec != nil || got.ForceRefresh || got.UseTLS {
		t.Fatalf("omitted fields must not be defaulted by the HTTP layer: %+v", got)
	}
}

func TestAnalyzeUsesTheAnalyzeBackendAndMapsItsErrors(t *testing.T) {
	var got service.ExtractInput
	h := handler(t, &stub{
		analyze: func(_ context.Context, in service.ExtractInput) (service.Outcome, error) {
			got = in
			return cachedOutcome(), nil
		},
	}, nil)
	rec := do(h, "POST", "/analyze", `{"query":"Kepler-186","run_id":"r1","star_id":"mine"}`)
	if rec.Code != 200 || got.RunID != "r1" || got.StarID != "mine" {
		t.Fatalf("status %d input %+v", rec.Code, got)
	}

	noModel := handler(t, &stub{analyze: func(context.Context, service.ExtractInput) (service.Outcome, error) {
		return service.Outcome{}, &service.Error{Kind: service.KindUnavailable, Message: "no trained model is available yet"}
	}}, nil)
	rec = do(noModel, "POST", "/analyze", `{"query":"Kepler-186"}`)
	if rec.Code != 503 || decodeError(t, rec).Error.Message != "no trained model is available yet" {
		t.Fatalf("no model must be a clear 503, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestAFullQueueIs503WithRetryAfter(t *testing.T) {
	h := handler(t, &stub{extract: func(context.Context, service.ExtractInput) (service.Outcome, error) {
		return service.Outcome{}, &service.Error{Kind: service.KindBusy, Message: "the queue is full; try again in a few minutes"}
	}}, nil)
	rec := do(h, "POST", "/pipeline/extract", `{"query":"Kepler-186"}`)
	if rec.Code != 503 || rec.Header().Get("Retry-After") != "30" || decodeError(t, rec).Error.Code != "busy" {
		t.Fatalf("status %d headers %v body %s", rec.Code, rec.Header(), rec.Body.String())
	}
}

func TestExtractRejectsUnknownFieldsAndBadTypes(t *testing.T) {
	h := handler(t, &stub{}, nil)
	for name, body := range map[string]string{
		"unknown field":    `{"query":"x","sigma":3}`,
		"string sigma":     `{"query":"x","sigma_clip":"3"}`,
		"number force":     `{"query":"x","force_refresh":1}`,
		"object for query": `{"query":{"a":1}}`,
	} {
		rec := do(h, "POST", "/pipeline/extract", body)
		if rec.Code != 400 {
			t.Errorf("%s: status %d body %s", name, rec.Code, rec.Body.String())
		}
	}
}
