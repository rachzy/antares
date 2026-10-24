export const API_ROUTES = {
  health: '/healthz',
  resolveTarget: '/targets/resolve',
  starInfo: '/targets/info',
  listRuns: '/ml/runs',
  runHyperparameters: (runId: string) =>
    `/ml/runs/${encodeURIComponent(runId)}/hyperparameters`,
  runDataset: (runId: string) =>
    `/ml/runs/${encodeURIComponent(runId)}/dataset`,
  predict: '/ml/predict',
  extract: '/pipeline/extract',
  analyze: '/analyze',
  subscription: (token: string) => `/subscription/${encodeURIComponent(token)}`,
} as const;
