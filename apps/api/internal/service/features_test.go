package service_test

import (
	"context"
	"strings"
	"testing"

	"antares/api/internal/jobs"
	"antares/api/internal/service"
)

func TestLatestFeaturesByName(t *testing.T) {
	e := newEnv(t, nil)
	target := e.seedFeatures(t, "Kepler-186", defaultParams)

	view, err := e.svc.LatestFeatures(context.Background(), service.ResolveInput{Query: "kepler 186"})
	if err != nil {
		t.Fatal(err)
	}
	if view.Target.CatalogID != target.CatalogID || view.ShaulaVersion != "0.0.1a0" || view.CreatedAt.IsZero() {
		t.Fatalf("view = %+v", view)
	}
	if len(view.Features) != 1 || view.Features[0]["period_days"] != 129.9 {
		t.Fatalf("features = %+v", view.Features)
	}
}

func TestLatestFeaturesByPosition(t *testing.T) {
	e := newEnv(t, nil)
	target := e.seedFeatures(t, "Kepler-186", defaultParams)

	view, err := e.svc.LatestFeatures(context.Background(), service.ResolveInput{RA: floatPtr(target.RA), Dec: floatPtr(target.Dec)})
	if err != nil || view.Target.CatalogID != target.CatalogID || len(view.Features) != 1 {
		t.Fatalf("view = %+v, %v", view, err)
	}
}

func TestLatestFeaturesByToken(t *testing.T) {
	e := newEnv(t, nil)
	out, err := e.svc.Extract(context.Background(), service.ExtractInput{Query: "Kepler-186"})
	if err != nil || out.Token == "" {
		t.Fatalf("Extract = %+v, %v", out, err)
	}
	target := e.seedFeatures(t, "Kepler-186", defaultParams)

	view, err := e.svc.LatestFeaturesByToken(context.Background(), out.Token)
	if err != nil || view.Target.CatalogID != target.CatalogID || len(view.Features) != 1 {
		t.Fatalf("view = %+v, %v", view, err)
	}
}

func TestLatestFeaturesNeverExtractedIsNotFound(t *testing.T) {
	e := newEnv(t, nil)
	_, err := e.svc.LatestFeatures(context.Background(), service.ResolveInput{Query: "Kepler-186"})
	if err == nil || kindOf(t, err) != service.KindNotFound || !strings.Contains(err.Error(), "/pipeline/extract") {
		t.Fatalf("err = %v", err)
	}

	out, err := e.svc.Extract(context.Background(), service.ExtractInput{Query: "Kepler-186"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.svc.LatestFeaturesByToken(context.Background(), out.Token)
	if err == nil || kindOf(t, err) != service.KindNotFound || !strings.Contains(err.Error(), "/pipeline/extract") {
		t.Fatalf("by token err = %v", err)
	}
}

func TestLatestFeaturesUnknownTokenIsNotFound(t *testing.T) {
	e := newEnv(t, nil)
	valid, _ := jobs.NewToken()
	for _, token := range []string{valid, "abc", ""} {
		if _, err := e.svc.LatestFeaturesByToken(context.Background(), token); err == nil || kindOf(t, err) != service.KindNotFound {
			t.Errorf("token %q: %v", token, err)
		}
	}
}
