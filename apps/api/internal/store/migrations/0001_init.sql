CREATE TABLE targets (
  id           INTEGER PRIMARY KEY,
  catalog      TEXT NOT NULL,
  catalog_id   TEXT NOT NULL,
  ra_deg       REAL NOT NULL,
  dec_deg      REAL NOT NULL,
  display_name TEXT NOT NULL,
  designation  TEXT NOT NULL,
  created_at   TEXT NOT NULL,
  UNIQUE (catalog, catalog_id)
);
CREATE INDEX targets_dec ON targets (catalog, dec_deg);

CREATE TABLE target_aliases (
  alias     TEXT NOT NULL,
  catalog   TEXT NOT NULL,
  target_id INTEGER NOT NULL REFERENCES targets (id),
  PRIMARY KEY (alias, catalog)
);

CREATE TABLE feature_cache (
  id             INTEGER PRIMARY KEY,
  target_id      INTEGER NOT NULL REFERENCES targets (id),
  params_hash    TEXT NOT NULL,
  shaula_version TEXT NOT NULL,
  features_json  TEXT NOT NULL,
  features_hash  TEXT NOT NULL,
  created_at     TEXT NOT NULL,
  expires_at     TEXT,
  superseded_at  TEXT,
  UNIQUE (target_id, params_hash, shaula_version)
);

CREATE TABLE prediction_cache (
  id               INTEGER PRIMARY KEY,
  features_hash    TEXT NOT NULL,
  model_run_id     TEXT NOT NULL,
  schema_version   INTEGER NOT NULL,
  predictions_json TEXT NOT NULL,
  target_id        INTEGER REFERENCES targets (id),
  created_at       TEXT NOT NULL,
  UNIQUE (features_hash, model_run_id)
);

CREATE TABLE jobs (
  id           TEXT PRIMARY KEY,
  dedupe_key   TEXT NOT NULL,
  kind         TEXT NOT NULL,
  state        TEXT NOT NULL,
  target_id    INTEGER NOT NULL REFERENCES targets (id),
  request_json TEXT NOT NULL,
  result_json  TEXT,
  error        TEXT,
  created_at   TEXT NOT NULL,
  started_at   TEXT,
  finished_at  TEXT
);
CREATE UNIQUE INDEX jobs_one_active_per_key ON jobs (dedupe_key) WHERE state IN ('queued', 'running');

CREATE TABLE subscriptions (
  token_hash TEXT PRIMARY KEY,
  job_id     TEXT NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
  created_at TEXT NOT NULL,
  expires_at TEXT NOT NULL
);
CREATE INDEX subscriptions_job ON subscriptions (job_id);
CREATE INDEX subscriptions_expiry ON subscriptions (expires_at);

CREATE TABLE job_milestones (
  job_id  TEXT NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
  seq     INTEGER NOT NULL,
  stage   TEXT NOT NULL,
  message TEXT NOT NULL,
  at      TEXT NOT NULL,
  PRIMARY KEY (job_id, seq)
);
