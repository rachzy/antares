package httpapi_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"antares/api/internal/jobs"
	"antares/api/internal/service"
)

func sampleView() service.FeaturesView {
	return service.FeaturesView{
		Target:        jobs.TargetInfo{Catalog: "KIC", CatalogID: "8120608", DisplayName: "Kepler-186", Designation: "KIC 8120608"},
		ShaulaVersion: "0.0.1a0",
		CreatedAt:     time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC),
		Features:      []map[string]any{{"period_days": 129.9}},
	}
}

func TestLatestFeaturesByNameForwardsTheInput(t *testing.T) {
	var got service.ResolveInput
	h := handler(t, &stub{latestFeatures: func(_ context.Context, in service.ResolveInput) (service.FeaturesView, error) {
		got = in
		return sampleView(), nil
	}}, nil)

	rec := do(h, "GET", "/targets/features?query=Kepler-186&mission=TESS", "")
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["shaula_version"] != "0.0.1a0" || body["target"].(map[string]any)["catalog_id"] != "8120608" {
		t.Fatalf("body = %v", body)
	}
	if got.Query != "Kepler-186" || got.Mission != "TESS" || got.RA != nil || got.Dec != nil {
		t.Fatalf("service input = %+v", got)
	}
}

func TestLatestFeaturesByPositionParsesCoordinates(t *testing.T) {
	var got service.ResolveInput
	h := handler(t, &stub{latestFeatures: func(_ context.Context, in service.ResolveInput) (service.FeaturesView, error) {
		got = in
		return sampleView(), nil
	}}, nil)

	rec := do(h, "GET", "/targets/features?ra_deg=298.65&dec_deg=-43.9", "")
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if got.RA == nil || *got.RA != 298.65 || got.Dec == nil || *got.Dec != -43.9 || got.Query != "" {
		t.Fatalf("service input = %+v", got)
	}
}

func TestLatestFeaturesRejectsBadCoordinates(t *testing.T) {
	h := handler(t, &stub{}, nil)
	for _, q := range []string{"ra_deg=abc&dec_deg=1", "ra_deg=1&dec_deg=x", "ra_deg=NaN&dec_deg=1", "ra_deg=1e999&dec_deg=1"} {
		rec := do(h, "GET", "/targets/features?"+q, "")
		var body errorBody
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if rec.Code != 400 || body.Error.Code != "invalid_request" {
			t.Errorf("%s: status %d, body %s", q, rec.Code, rec.Body)
		}
	}
}

func TestLatestFeaturesMapsServiceErrors(t *testing.T) {
	h := handler(t, &stub{latestFeatures: func(context.Context, service.ResolveInput) (service.FeaturesView, error) {
		return service.FeaturesView{}, &service.Error{Kind: service.KindNotFound, Message: "no features yet"}
	}}, nil)
	rec := do(h, "GET", "/targets/features?query=x", "")
	var body errorBody
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != 404 || body.Error.Code != "not_found" {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body)
	}
}

func TestLatestFeaturesByTokenReturnsTheView(t *testing.T) {
	token := newToken(t)
	var got string
	h := handler(t, &stub{latestFeaturesByToken: func(_ context.Context, tok string) (service.FeaturesView, error) {
		got = tok
		return sampleView(), nil
	}}, nil)
	rec := do(h, "GET", "/subscription/"+token+"/features", "")
	if rec.Code != 200 || got != token {
		t.Fatalf("status %d, token %q: %s", rec.Code, got, rec.Body)
	}
}

func TestLatestFeaturesByTokenRejectsMalformedTokensBeforeTheBackend(t *testing.T) {
	h := handler(t, &stub{}, nil) // an unexpected backend call would surface as a 500
	rec := do(h, "GET", "/subscription/"+strings.Repeat("a", 5)+"/features", "")
	if rec.Code != 404 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
}

func TestLatestFeaturesByTokenMapsServiceErrors(t *testing.T) {
	h := handler(t, &stub{latestFeaturesByToken: func(context.Context, string) (service.FeaturesView, error) {
		return service.FeaturesView{}, &service.Error{Kind: service.KindInternal, Message: "db exploded"}
	}}, nil)
	rec := do(h, "GET", "/subscription/"+newToken(t)+"/features", "")
	if rec.Code != 500 || strings.Contains(rec.Body.String(), "db exploded") {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
}
