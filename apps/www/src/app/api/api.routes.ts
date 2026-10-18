export const API_ROUTES = {
  health: '/healthz',
  resolveTarget: '/targets/resolve',
  listRuns: '/ml/runs',
  predict: '/ml/predict',
  extract: '/pipeline/extract',
  analyze: '/analyze',
  subscription: (token: string) => `/subscription/${encodeURIComponent(token)}`,
} as const;
