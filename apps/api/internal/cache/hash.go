// Package cache stores extracted features and model predictions in SQLite, keyed by content hashes
// so equal inputs are served from the cache whichever request produced them.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// Sanitize returns a copy of v with every NaN or infinite float replaced by nil, recursing into
// map[string]any and []any; other values are returned as is. encoding/json rejects non-finite
// numbers, so rows are sanitized before they are stored, hashed or returned.
func Sanitize(v any) any {
	switch x := v.(type) {
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return nil
		}
		return x
	case float32:
		if f := float64(x); math.IsNaN(f) || math.IsInf(f, 0) {
			return nil
		}
		return x
	case map[string]any:
		out := make(map[string]any, len(x))
		for key, item := range x {
			out[key] = Sanitize(item)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = Sanitize(item)
		}
		return out
	default:
		return v
	}
}

// SanitizeRows applies Sanitize to every row, returning new rows.
func SanitizeRows(rows []map[string]any) []map[string]any {
	out := make([]map[string]any, len(rows))
	for i, row := range rows {
		out[i] = Sanitize(row).(map[string]any)
	}
	return out
}

// ExtractParams are the normalised extraction settings, mirroring Shaula's ExtractFeaturesRequest.
// Everything that changes Shaula's output for a target belongs here, since Hash keys the feature
// cache and jobs on it.
type ExtractParams struct {
	Mission      string  `json:"mission"`
	SigmaClip    float64 `json:"sigma_clip"`
	DownloadAll  bool    `json:"download_all"`
	Author       string  `json:"author"`
	Exptime      string  `json:"exptime"`
	UseTLS       bool    `json:"use_tls"`
	MaskEclipses bool    `json:"mask_eclipses"`
}

// Hash returns a hex SHA-256 of the parameters in a fixed field order. The canonical form is
// hand-written so the key does not change with struct tags or field order; %q keeps strings
// unambiguous and %v prints SigmaClip in its shortest exact form, so equal values always hash
// alike.
func (p ExtractParams) Hash() string {
	canonical := fmt.Sprintf("%q|%v|%t|%q|%q|%t|%t",
		p.Mission, p.SigmaClip, p.DownloadAll, p.Author, p.Exptime, p.UseTLS, p.MaskEclipses)
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

// FeaturesHash returns a hex SHA-256 of the sanitized rows' JSON, which keys the prediction cache.
// encoding/json writes map keys sorted, so the digest depends only on the rows' content, and
// non-finite values hash as the null they are stored as.
func FeaturesHash(rows []map[string]any) (string, error) {
	body, err := json.Marshal(SanitizeRows(rows))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

// ExpiresAt returns when features of mission extracted at now go stale: after tessTTL for TESS,
// whose sectors keep growing, and never (nil) for the finished Kepler and K2 missions.
func ExpiresAt(mission string, now time.Time, tessTTL time.Duration) *time.Time {
	if mission != "TESS" {
		return nil
	}
	t := now.Add(tessTTL)
	return &t
}
