package catalog

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Default endpoints of the two public TAP services.
const (
	DefaultSimbadURL = "https://simbad.cds.unistra.fr/simbad/sim-tap/sync"
	DefaultNASAURL   = "https://exoplanetarchive.ipac.caltech.edu/TAP/sync"

	lightYearsPerParsec = 3.26156
	requestTimeout      = 10 * time.Second
)

// Client is the Source backed by SIMBAD and the NASA Exoplanet Archive.
type Client struct {
	SimbadURL string
	NASAURL   string
	HTTP      *http.Client
}

// NewClient returns a Client for the given endpoints, using the defaults for empty ones.
func NewClient(simbadURL, nasaURL string) *Client {
	if simbadURL == "" {
		simbadURL = DefaultSimbadURL
	}
	if nasaURL == "" {
		nasaURL = DefaultNASAURL
	}
	return &Client{SimbadURL: simbadURL, NASAURL: nasaURL, HTTP: &http.Client{Timeout: requestTimeout}}
}

// Lookup asks both services and merges what they know. It fails only when both fail.
func (c *Client) Lookup(ctx context.Context, q Query) (Star, error) {
	if err := q.validate(); err != nil {
		return Star{}, err
	}
	star := Star{Aliases: []string{}, Planets: []Planet{}, Sources: []string{}}

	simbadErr := c.simbad(ctx, q, &star)
	nasaErr := c.nasa(ctx, q, &star)
	if simbadErr != nil && nasaErr != nil {
		return Star{}, errors.Join(simbadErr, nasaErr)
	}
	return star, nil
}

func (c *Client) simbad(ctx context.Context, q Query, star *Star) error {
	rows, err := simbadRows(ctx, c.HTTP, c.SimbadURL, fmt.Sprintf(
		"SELECT b.oid, b.sp_type, b.otype, b.plx_value FROM basic AS b JOIN ident AS i ON i.oidref = b.oid WHERE i.id = '%s'",
		q.designation()))
	if err != nil {
		return fmt.Errorf("simbad: %w", err)
	}
	if len(rows) == 0 || len(rows[0]) < 4 {
		return nil
	}
	oid, ok := rows[0][0].(float64)
	if !ok {
		return nil
	}
	star.Sources = append(star.Sources, "SIMBAD")
	star.SpectralType = text(rows[0][1])
	star.ObjectType = text(rows[0][2])
	if plx := number(rows[0][3]); plx != nil && *plx > 0 {
		ly := 1000 / *plx * lightYearsPerParsec
		star.DistanceLY = &ly
	}

	// The follow-up queries only enrich the star, so their failures leave it as it is.
	if ids, err := simbadRows(ctx, c.HTTP, c.SimbadURL, fmt.Sprintf("SELECT id FROM ident WHERE oidref = %d", int64(oid))); err == nil {
		for _, row := range ids {
			if len(row) > 0 {
				if alias := friendlyAlias(text(row[0]), q); alias != "" {
					star.Aliases = append(star.Aliases, alias)
				}
			}
		}
	}
	if fluxes, err := simbadRows(ctx, c.HTTP, c.SimbadURL, fmt.Sprintf("SELECT filter, flux FROM flux WHERE oidref = %d AND filter IN ('G', 'V')", int64(oid))); err == nil {
		bands := map[string]float64{}
		for _, row := range fluxes {
			if len(row) == 2 {
				if mag := number(row[1]); mag != nil {
					bands[text(row[0])] = *mag
				}
			}
		}
		for _, band := range []string{"G", "V"} {
			if mag, ok := bands[band]; ok {
				star.Magnitude, star.MagnitudeBand = &mag, band
				break
			}
		}
	}
	return nil
}

// aliasPrefixes are the designations people recognise; SIMBAD lists many survey-internal ones.
var aliasPrefixes = []string{"Kepler-", "K2-", "TOI-", "KOI-", "HD ", "HIP ", "GJ ", "WASP-", "HAT-P-"}

// friendlyAlias returns id without SIMBAD's "* " and "V* " object markers if it is a designation
// worth showing, and "" otherwise.
func friendlyAlias(id string, q Query) string {
	id = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(id, "V*"), "*"))
	if id == q.designation() {
		return ""
	}
	for _, p := range aliasPrefixes {
		if strings.HasPrefix(id, p) {
			return id
		}
	}
	return ""
}

func (c *Client) nasa(ctx context.Context, q Query, star *Star) error {
	var adql string
	switch q.Catalog {
	case "KIC":
		adql = fmt.Sprintf("SELECT kepoi_name, kepler_name, koi_disposition, koi_period, koi_prad, koi_steff, koi_srad, koi_kepmag FROM cumulative WHERE kepid = %s", q.ID)
	case "TIC":
		adql = fmt.Sprintf("SELECT pl_name, pl_orbper, pl_rade, st_teff, st_rad, st_mass, sy_dist FROM pscomppars WHERE tic_id = '%s'", q.designation())
	default:
		return nil // the archive has no EPIC lookup
	}
	rows, err := nasaRows(ctx, c.HTTP, c.NASAURL, adql)
	if err != nil {
		return fmt.Errorf("nasa: %w", err)
	}
	if len(rows) == 0 {
		return nil
	}
	star.Sources = append(star.Sources, "NASA Exoplanet Archive")

	host := ""
	for _, row := range rows {
		if q.Catalog == "KIC" {
			star.TeffK = firstNumber(star.TeffK, row["koi_steff"])
			if r := number(row["koi_srad"]); r != nil {
				star.RadiusSolar = firstNumber(star.RadiusSolar, r)
			}
			if star.Magnitude == nil {
				if mag := number(row["koi_kepmag"]); mag != nil {
					star.Magnitude, star.MagnitudeBand = mag, "Kepler"
				}
			}
			disposition := strings.ToLower(text(row["koi_disposition"]))
			if disposition != "confirmed" && disposition != "candidate" {
				continue
			}
			name := text(row["kepler_name"])
			if name == "" {
				name = text(row["kepoi_name"])
			} else if host == "" {
				host = hostOf(name)
			}
			star.Planets = append(star.Planets, Planet{
				Name: name, PeriodDays: number(row["koi_period"]), RadiusEarth: number(row["koi_prad"]), Confirmation: disposition,
			})
			continue
		}
		star.TeffK = firstNumber(star.TeffK, row["st_teff"])
		star.RadiusSolar = firstNumber(star.RadiusSolar, row["st_rad"])
		star.MassSolar = firstNumber(star.MassSolar, row["st_mass"])
		if star.DistanceLY == nil {
			if pc := number(row["sy_dist"]); pc != nil {
				ly := *pc * lightYearsPerParsec
				star.DistanceLY = &ly
			}
		}
		star.Planets = append(star.Planets, Planet{
			Name: text(row["pl_name"]), PeriodDays: number(row["pl_orbper"]), RadiusEarth: number(row["pl_rade"]), Confirmation: "confirmed",
		})
	}
	sort.SliceStable(star.Planets, func(i, j int) bool { return less(star.Planets[i].PeriodDays, star.Planets[j].PeriodDays) })

	// The KOI table has no stellar mass; the confirmed-planet table does, keyed by host name.
	if host != "" && hostName.MatchString(host) && star.MassSolar == nil {
		if rows, err := nasaRows(ctx, c.HTTP, c.NASAURL, fmt.Sprintf("SELECT st_mass, sy_dist FROM pscomppars WHERE hostname = '%s'", host)); err == nil && len(rows) > 0 {
			star.MassSolar = number(rows[0]["st_mass"])
			if pc := number(rows[0]["sy_dist"]); pc != nil && star.DistanceLY == nil {
				ly := *pc * lightYearsPerParsec
				star.DistanceLY = &ly
			}
		}
	}
	return nil
}

// firstNumber keeps cur when set, otherwise it converts v.
func firstNumber(cur *float64, v any) *float64 {
	if cur != nil {
		return cur
	}
	if p, ok := v.(*float64); ok {
		return p
	}
	return number(v)
}

// hostOf returns "Kepler-90" for "Kepler-90 b".
func hostOf(planet string) string {
	i := strings.LastIndex(planet, " ")
	if i < 1 {
		return ""
	}
	return planet[:i]
}

// less orders planets by period, unknown periods last.
func less(a, b *float64) bool {
	switch {
	case a == nil:
		return false
	case b == nil:
		return true
	}
	return *a < *b
}
