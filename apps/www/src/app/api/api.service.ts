import { HttpClient, HttpErrorResponse } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, catchError, throwError } from 'rxjs';
import { API_BASE_URL } from './api.config';
import { API_ROUTES } from './api.routes';
import type {
  AnalyzeRequest,
  ApiErrorCode,
  ExtractRequest,
  Health,
  Job,
  PredictRequest,
  PredictResult,
  ResolveRequest,
  Run,
  SubscriptionEvent,
  SubscriptionView,
  StarInfo,
  Target,
} from './api.types';

export class ApiRequestError extends Error {
  constructor(
    readonly status: number,
    readonly code: ApiErrorCode,
    message: string,
    readonly retryAfter?: number,
  ) {
    super(message);
  }
}

function toApiError(err: unknown): ApiRequestError {
  if (!(err instanceof HttpErrorResponse)) {
    return new ApiRequestError(0, 'unavailable', 'request failed');
  }
  const envelope = err.error?.error;
  const retry = Number(err.headers?.get('Retry-After'));
  const retryAfter = Number.isFinite(retry) && retry > 0 ? retry : undefined;
  if (envelope && typeof envelope.code === 'string') {
    return new ApiRequestError(
      err.status,
      envelope.code,
      String(envelope.message ?? err.message),
      retryAfter,
    );
  }
  return new ApiRequestError(
    err.status,
    err.status >= 500 || err.status === 0 ? 'unavailable' : 'internal',
    err.message,
    retryAfter,
  );
}

@Injectable({ providedIn: 'root' })
export class ApiService {
  private readonly http = inject(HttpClient);
  private readonly base = inject(API_BASE_URL);

  health(): Observable<Health> {
    return this.get<Health>(API_ROUTES.health);
  }

  resolveTarget(body: ResolveRequest): Observable<Target> {
    return this.post<Target>(API_ROUTES.resolveTarget, body);
  }

  starInfo(query: string): Observable<StarInfo> {
    return this.get<StarInfo>(API_ROUTES.starInfo, { query });
  }

  listRuns(): Observable<Run[]> {
    return this.get<Run[]>(API_ROUTES.listRuns);
  }

  predict(body: PredictRequest): Observable<PredictResult> {
    return this.post<PredictResult>(API_ROUTES.predict, body);
  }

  extract(body: ExtractRequest): Observable<Job> {
    return this.post<Job>(API_ROUTES.extract, body);
  }

  analyze(body: AnalyzeRequest): Observable<Job> {
    return this.post<Job>(API_ROUTES.analyze, body);
  }

  snapshot(token: string): Observable<SubscriptionView> {
    return this.get<SubscriptionView>(API_ROUTES.subscription(token));
  }

  watch(token: string, since?: number): Observable<SubscriptionEvent[]> {
    return new Observable((subscriber) => {
      const query = since === undefined ? '' : `?since=${since}`;
      const url = this.base.replace(/^http/, 'ws');
      const ws = new WebSocket(url + API_ROUTES.subscription(token) + query);
      ws.onmessage = (e) => {
        try {
          subscriber.next(JSON.parse(e.data).events ?? []);
        } catch {
          subscriber.error({ code: 1007 });
        }
      };
      ws.onclose = (e) =>
        e.code === 1000
          ? subscriber.complete()
          : subscriber.error({ code: e.code });
      return () => {
        ws.onmessage = null;
        ws.onclose = null;
        ws.close();
      };
    });
  }

  private get<T>(path: string, params?: Record<string, string>): Observable<T> {
    return this.http
      .get<T>(this.base + path, { params })
      .pipe(catchError((e) => throwError(() => toApiError(e))));
  }

  private post<T>(path: string, body: unknown): Observable<T> {
    return this.http
      .post<T>(this.base + path, body)
      .pipe(catchError((e) => throwError(() => toApiError(e))));
  }
}
