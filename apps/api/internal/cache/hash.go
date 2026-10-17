package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"time"
)

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

func SanitizeRows(rows []map[string]any) []map[string]any {
	out := make([]map[string]any, len(rows))
	for i, row := range rows {
		out[i] = Sanitize(row).(map[string]any)
	}
	return out
}

type ExtractParams struct {
	Mission      string  `json:"mission"`
	SigmaClip    float64 `json:"sigma_clip"`
	DownloadAll  bool    `json:"download_all"`
	Author       string  `json:"author"`
	Exptime      string  `json:"exptime"`
	UseTLS       bool    `json:"use_tls"`
	MaskEclipses bool    `json:"mask_eclipses"`
}

func (p ExtractParams) Hash() string {
	canonical := fmt.Sprintf("%q|%v|%t|%q|%q|%t|%t",
		p.Mission, p.SigmaClip, p.DownloadAll, p.Author, p.Exptime, p.UseTLS, p.MaskEclipses)
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

func FeaturesHash(rows []map[string]any) (string, error) {
	body, err := json.Marshal(SanitizeRows(rows))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func ExpiresAt(mission string, now time.Time, tessTTL time.Duration) *time.Time {
	if mission != "TESS" {
		return nil
	}
	t := now.Add(tessTTL)
	return &t
}
