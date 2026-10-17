package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

type Target struct {
	ID          int64
	Catalog     string
	CatalogID   string
	RA          float64
	Dec         float64
	DisplayName string
	Designation string
}

type Targets struct{ DB *sql.DB }

type scanner interface{ Scan(dest ...any) error }

func scanTarget(row scanner) (Target, error) {
	var t Target
	err := row.Scan(&t.ID, &t.Catalog, &t.CatalogID, &t.RA, &t.Dec, &t.DisplayName, &t.Designation)
	return t, err
}

func NormalizeAlias(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if r == ' ' || r == '-' || r == '_' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func (t Targets) Upsert(ctx context.Context, in Target, now time.Time, aliases ...string) (Target, error) {
	tx, err := t.DB.BeginTx(ctx, nil)
	if err != nil {
		return Target{}, err
	}
	defer func() { _ = tx.Rollback() }()

	out := in
	err = tx.QueryRowContext(ctx, `
		INSERT INTO targets (catalog, catalog_id, ra_deg, dec_deg, display_name, designation, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (catalog, catalog_id) DO UPDATE SET designation = excluded.designation
		RETURNING id, ra_deg, dec_deg, display_name`,
		in.Catalog, in.CatalogID, in.RA, in.Dec, in.DisplayName, in.Designation, FormatTime(now),
	).Scan(&out.ID, &out.RA, &out.Dec, &out.DisplayName)
	if err != nil {
		return Target{}, err
	}

	for _, alias := range append([]string{in.DisplayName, in.Designation}, aliases...) {
		normalised := NormalizeAlias(alias)
		if normalised == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO target_aliases (alias, catalog, target_id) VALUES (?, ?, ?)
			ON CONFLICT (alias, catalog) DO UPDATE SET target_id = excluded.target_id`,
			normalised, in.Catalog, out.ID); err != nil {
			return Target{}, err
		}
	}
	return out, tx.Commit()
}

func (t Targets) ByAlias(ctx context.Context, alias, catalog string) (Target, bool, error) {
	row := t.DB.QueryRowContext(ctx, `
		SELECT t.id, t.catalog, t.catalog_id, t.ra_deg, t.dec_deg, t.display_name, t.designation
		FROM target_aliases a JOIN targets t ON t.id = a.target_id
		WHERE a.alias = ? AND a.catalog = ?`, NormalizeAlias(alias), catalog)
	got, err := scanTarget(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Target{}, false, nil
	}
	if err != nil {
		return Target{}, false, err
	}
	return got, true, nil
}
