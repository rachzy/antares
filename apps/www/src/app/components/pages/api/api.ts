import { Component, inject } from '@angular/core';
import { HlmTableImports } from '@spartan-ng/helm/table';
import { API_BASE_URL } from '../../../api/api.config';

interface Step {
  title: string;
  service: string;
  text: string;
  code: string;
}

const ENDPOINTS = [
  {
    method: 'POST',
    path: '/targets/resolve',
    text: 'Find a star by name or position',
    service: 'Shaula',
  },
  {
    method: 'GET',
    path: '/targets/info',
    text: 'Catalogue details and known planets',
    service: 'Shaula',
  },
  {
    method: 'GET',
    path: '/targets/features',
    text: 'Latest cached features of a star',
    service: 'Shaula',
  },
  {
    method: 'POST',
    path: '/pipeline/extract',
    text: 'Extract features, from the cache or a new job',
    service: 'Shaula',
  },
  {
    method: 'GET',
    path: '/ml/runs',
    text: 'List the trained model runs',
    service: 'Fang',
  },
  {
    method: 'GET',
    path: '/ml/runs/{run_id}/hyperparameters',
    text: 'Threshold and tuned hyperparameters of a run',
    service: 'Fang',
  },
  {
    method: 'GET',
    path: '/ml/runs/{run_id}/dataset',
    text: 'Label counts and confusion matrices of a run',
    service: 'Fang',
  },
  {
    method: 'POST',
    path: '/ml/predict',
    text: 'Score feature rows with a run',
    service: 'Fang',
  },
  {
    method: 'POST',
    path: '/analyze',
    text: 'Extract, then score',
    service: 'Both',
  },
  {
    method: 'GET',
    path: '/subscription/{token}',
    text: 'Job snapshot, or a WebSocket stream of its progress',
    service: 'Both',
  },
];

@Component({
  selector: 'an-api',
  imports: [HlmTableImports],
  templateUrl: './api.html',
  host: { class: 'block p-6 md:p-10' },
})
export class Api {
  protected readonly base = inject(API_BASE_URL);
  protected readonly endpoints = ENDPOINTS;
  protected readonly steps: Step[] = [
    {
      title: 'Extract features from a star',
      service: 'Shaula',
      text: 'Send a star name, or a position, and Antares finds its light curve and measures transit features. A star processed before answers right away (200). Otherwise a job is queued (202) and you get a token.',
      code: `curl -X POST ${this.base}/pipeline/extract \\
  -H "Content-Type: application/json" \\
  -d '{"query": "Kepler-186", "mission": "Kepler"}'

# 202 -> {"status": "queued", "token": "q3Zr8y...", "position": 1}`,
    },
    {
      title: 'Follow the job',
      service: 'Shaula',
      text: 'Extraction takes minutes. Open a WebSocket on the token to stream progress, or poll the same path for a JSON snapshot.',
      code: `GET ${this.base}/subscription/{token}            # snapshot, or WebSocket with Upgrade
GET ${this.base}/subscription/{token}/features   # latest features of the star`,
    },
    {
      title: 'Classify the features',
      service: 'Fang',
      text: "Send the feature rows to a trained model run. Each row comes back with the model's score. Leave run_id out to use the newest run.",
      code: `curl -X POST ${this.base}/ml/predict \\
  -H "Content-Type: application/json" \\
  -d '{"features": [{"period_days": 129.9, "MES": 12.5}], "star_id": "KIC 8120608"}'`,
    },
    {
      title: 'Or do both in one call',
      service: 'Shaula and Fang',
      text: '/analyze extracts the features, then scores them. It queues and streams exactly like step 1.',
      code: `curl -X POST ${this.base}/analyze \\
  -H "Content-Type: application/json" \\
  -d '{"query": "Kepler-186"}'`,
    },
  ];
}
