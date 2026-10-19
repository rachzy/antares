package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const maxResponseBytes = 1 << 20

// tap runs one ADQL query against a TAP sync endpoint and returns the JSON body.
func tap(ctx context.Context, hc *http.Client, endpoint string, form url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s answered %d", endpoint, resp.StatusCode)
	}
	if len(body) > maxResponseBytes {
		return nil, fmt.Errorf("%s answered more than %d bytes", endpoint, maxResponseBytes)
	}
	return body, nil
}

// simbadRows runs an ADQL query on SIMBAD, whose JSON is {"metadata": [...], "data": [[...]]}.
func simbadRows(ctx context.Context, hc *http.Client, endpoint, adql string) ([][]any, error) {
	body, err := tap(ctx, hc, endpoint, url.Values{
		"REQUEST": {"doQuery"}, "LANG": {"ADQL"}, "FORMAT": {"json"}, "QUERY": {adql},
	})
	if err != nil {
		return nil, err
	}
	var out struct {
		Data [][]any `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("simbad: decode: %w", err)
	}
	return out.Data, nil
}

// nasaRows runs an ADQL query on the NASA Exoplanet Archive, whose JSON is an array of objects.
func nasaRows(ctx context.Context, hc *http.Client, endpoint, adql string) ([]map[string]any, error) {
	body, err := tap(ctx, hc, endpoint, url.Values{"query": {adql}, "format": {"json"}})
	if err != nil {
		return nil, err
	}
	var rows []map[string]any
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, fmt.Errorf("nasa: decode: %w", err)
	}
	return rows, nil
}

func number(v any) *float64 {
	f, ok := v.(float64)
	if !ok {
		return nil
	}
	return &f
}

func text(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}
