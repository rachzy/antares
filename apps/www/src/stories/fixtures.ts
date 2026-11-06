import type {
  FeatureRow,
  JobResult,
  PredictionRow,
  StarDetails,
  SubscriptionEvent,
  Target,
} from '../app/api/api.types';
import type { Stage } from '../app/stores/run.store';

export const target: Target = {
  catalog: 'KIC',
  catalog_id: '11446443',
  display_name: 'Kepler-1',
  designation: 'KIC 11446443',
  ra_deg: 286.9709,
  dec_deg: 49.3163,
};

export const starDetails: StarDetails = {
  aliases: ['TrES-2', 'KOI-1.01', 'GSC 03549-02811', 'TYC 3549-2811-1'],
  spectral_type: 'G0V',
  object_type: 'Star',
  distance_ly: 750,
  teff_k: 5850,
  radius_solar: 1.0,
  mass_solar: 0.98,
  magnitude: 11.4,
  magnitude_band: 'Kepler',
  planets: [
    {
      name: 'Kepler-1 b',
      period_days: 2.47063,
      radius_earth: 14.4,
      confirmation: 'confirmed',
    },
    { name: 'KOI-1.02', confirmation: 'candidate' },
  ],
  sources: ['SIMBAD', 'NASA Exoplanet Archive'],
};

const t0 = Date.parse('2026-11-06T14:02:11Z');
let seq = 0;
const ev = (
  secs: number,
  type: SubscriptionEvent['type'],
  stage: string,
  message?: string,
  fraction?: number,
): SubscriptionEvent => ({
  seq: ++seq,
  type,
  stage,
  message,
  fraction,
  at: new Date(t0 + secs * 1000).toISOString(),
});

const downloadEvents = (): SubscriptionEvent[] => [
  ev(0, 'state', 'queued', 'waiting for a worker'),
  ev(2, 'state', 'running', 'job started'),
  ev(3, 'progress', 'downloading', 'searching MAST for KIC 11446443'),
  ev(9, 'progress', 'downloading', 'downloading quarter 3', 0.35),
  ev(21, 'progress', 'downloading', 'downloading quarter 9', 0.7),
  ev(38, 'progress', 'downloading', 'stitching light curves', 1),
];

const processEvents = (): SubscriptionEvent[] => [
  ev(41, 'progress', 'detrending', 'removing stellar variability'),
  ev(95, 'progress', 'period_search', 'candidate 1: BLS peak at 2.4706 d', 0.3),
  ev(180, 'progress', 'period_search', 'candidate 1 passes MES gate', 0.6),
  ev(310, 'progress', 'period_search', 'candidate 2 rejected (MES 5.2)', 0.9),
  ev(322, 'progress', 'done', 'extraction finished', 1),
];

const predictEvents = (): SubscriptionEvent[] => [
  ev(330, 'state', 'queued', 'waiting for a worker'),
  ev(331, 'state', 'predicting', 'scoring 2 candidates'),
  ev(333, 'progress', 'done', 'scoring finished', 1),
];

export const features: FeatureRow[] = [
  {
    period_days: 2.470634,
    t0: 122.76,
    duration_hours: 1.8,
    depth_mean_per_transit: 0.0131,
    MES: 412.7,
    max_mes: 412.7,
    snr_global: 301.4,
    vshape_metric: 0.18,
    odd_even_depth_ratio: 1.01,
    secondary_depth_snr: 0.6,
    planet_radius_rearth: 14.4,
    detection_status: 'accepted',
  },
  {
    period_days: 7.911,
    t0: 124.3,
    duration_hours: 2.9,
    depth_mean_per_transit: 0.0004,
    MES: 6.4,
    max_mes: 6.4,
    snr_global: 6.9,
    vshape_metric: 0.74,
    odd_even_depth_ratio: 0.82,
    secondary_depth_snr: 2.1,
    planet_radius_rearth: 2.1,
    detection_status: 'provisional',
  },
];

export const predictions: PredictionRow[] = [
  {
    ...features[0],
    star_id: target.designation,
    prob_lightgbm: 0.97,
    prob_catboost: 0.95,
    prob_extratrees: 0.91,
    prob_stack: 0.96,
    potential_probability: 0.96,
    decision_threshold: 0.62,
    prediction: 'POTENTIAL',
    model_name: 'stack',
    model_run_id: '20261024-153012',
  },
  {
    ...features[1],
    star_id: target.designation,
    prob_lightgbm: 0.21,
    prob_catboost: 0.34,
    prob_extratrees: 0.4,
    prob_stack: 0.28,
    potential_probability: 0.28,
    decision_threshold: 0.62,
    prediction: 'UNLIKELY',
    model_name: 'stack',
    model_run_id: '20261024-153012',
  },
];

export const jobResult = (withPredictions: boolean): JobResult => ({
  target,
  cached: false,
  shaula_version: '0.0.1',
  features,
  ...(withPredictions
    ? {
        model_run_id: '20261024-153012',
        schema_version: 3,
        predictions,
      }
    : {}),
});

const idle: Stage = {
  status: 'idle',
  token: null,
  events: [],
  result: null,
  error: null,
  cached: false,
};

export const stage = (over: Partial<Stage>): Stage => ({
  ...idle,
  token: 'tok_demo',
  ...over,
});

export const events = {
  download: downloadEvents,
  process: processEvents,
  predict: predictEvents,
};
