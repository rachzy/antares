package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"antares/api/internal/db"
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
	q := db.New(tx)

	row, err := q.UpsertTarget(ctx, db.UpsertTargetParams{
		Catalog: in.Catalog, CatalogID: in.CatalogID, RaDeg: in.RA, DecDeg: in.Dec,
		DisplayName: in.DisplayName, Designation: in.Designation, CreatedAt: FormatTime(now),
	})
	if err != nil {
		return Target{}, err
	}
	out := in
	out.ID, out.RA, out.Dec, out.DisplayName = row.ID, row.RaDeg, row.DecDeg, row.DisplayName

	for _, alias := range append([]string{in.DisplayName, in.Designation}, aliases...) {
		normalised := NormalizeAlias(alias)
		if normalised == "" {
			continue
		}
		if err := q.UpsertAlias(ctx, db.UpsertAliasParams{Alias: normalised, Catalog: in.Catalog, TargetID: out.ID}); err != nil {
			return Target{}, err
		}
	}
	return out, tx.Commit()
}

func (t Targets) ByAlias(ctx context.Context, alias, catalog string) (Target, bool, error) {
	row, err := db.New(t.DB).TargetByAlias(ctx, db.TargetByAliasParams{Alias: NormalizeAlias(alias), Catalog: catalog})
	if errors.Is(err, sql.ErrNoRows) {
		return Target{}, false, nil
	}
	if err != nil {
		return Target{}, false, err
	}
	return Target{
		ID: row.ID, Catalog: row.Catalog, CatalogID: row.CatalogID, RA: row.RaDeg, Dec: row.DecDeg,
		DisplayName: row.DisplayName, Designation: row.Designation,
	}, true, nil
}
