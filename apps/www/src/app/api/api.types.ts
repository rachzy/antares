export type Mission = 'Kepler' | 'K2' | 'TESS';

export type ApiErrorCode =
  | 'invalid_request'
  | 'not_found'
  | 'too_large'
  | 'unprocessable'
  | 'unavailable'
  | 'busy'
  | 'internal';

export interface ApiError {
  error: { code: ApiErrorCode; message: string };
}

export interface Health {
  status: 'ok';
  ready: boolean;
  shaula_version: string;
  fang_version: string;
}

export interface ResolveRequest {
  query?: string;
  mission?: string;
  ra_deg?: number;
  dec_deg?: number;
}

export interface Target {
  catalog: string;
  catalog_id: string;
  display_name: string;
  designation: string;
  ra_deg: number;
  dec_deg: number;
}

export interface ExtractRequest {
  query?: string;
  ra_deg?: number;
  dec_deg?: number;
  mission?: string;
  sigma_clip?: number;
  download_all?: boolean;
  author?: string;
  exptime?: string;
  use_tls?: boolean;
  mask_eclipses?: boolean;
  force_refresh?: boolean;
  run_id?: string;
  star_id?: string;
}

export type AnalyzeRequest = ExtractRequest;

export type FeatureRow = Record<string, unknown>;
export type PredictionRow = Record<string, unknown>;

export interface JobResult {
  target: Target;
  cached: boolean;
  shaula_version: string;
  features: FeatureRow[];
  model_run_id?: string;
  schema_version?: number;
  predictions?: PredictionRow[];
}

export interface Done {
  status: 'done';
  result: JobResult;
}

export interface Queued {
  status: 'queued' | 'running';
  token: string;
  subscription_path: string;
  position: number;
}

export type Job = Done | Queued;

export interface PredictRequest {
  features: FeatureRow[];
  run_id?: string;
  star_id?: string;
}

export interface PredictResult {
  model_run_id: string;
  schema_version: number;
  predictions: PredictionRow[];
  cached: boolean;
}

export interface Run {
  run_id: string;
  created_at: string;
  schema_version: number;
  selected_model: string;
  threshold: number;
}

export interface SubscriptionEvent {
  seq: number;
  type: 'state' | 'progress' | 'result' | 'error';
  stage?: string;
  message?: string;
  fraction?: number;
  at: string;
  payload?: JobResult;
}

export interface Milestone {
  seq: number;
  stage: string;
  message: string;
  at: string;
}

export interface SubscriptionView {
  kind: 'extract' | 'analyze';
  state: 'queued' | 'running' | 'done' | 'failed' | 'dropped';
  position?: number;
  error?: string;
  result?: JobResult;
  milestones: Milestone[];
  last_seq: number;
}
