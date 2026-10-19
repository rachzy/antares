import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import {
  HttpTestingController,
  provideHttpClientTesting,
} from '@angular/common/http/testing';
import { ApiRequestError, ApiService } from './api.service';
import { API_BASE_URL } from './api.config';

class FakeSocket {
  static last: FakeSocket;
  onmessage: ((e: { data: string }) => void) | null = null;
  onclose: ((e: { code: number }) => void) | null = null;
  closed = false;
  constructor(public url: string) {
    FakeSocket.last = this;
  }
  close() {
    this.closed = true;
  }
}

describe('ApiService', () => {
  let api: ApiService;
  let http: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()],
    });
    api = TestBed.inject(ApiService);
    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => {
    http.verify();
    vi.unstubAllGlobals();
  });

  it('posts resolveTarget to the targets route', () => {
    const base = TestBed.inject(API_BASE_URL);
    const body = { query: 'Kepler-22' };
    let out: unknown;
    api.resolveTarget(body).subscribe((v) => (out = v));
    const req = http.expectOne(`${base}/targets/resolve`);
    expect(req.request.method).toBe('POST');
    expect(req.request.body).toEqual(body);
    req.flush({ designation: 'x' });
    expect(out).toEqual({ designation: 'x' });
  });

  it('gets starInfo from the targets/info route with the query', () => {
    const base = TestBed.inject(API_BASE_URL);
    let out: unknown;
    api.starInfo('KIC 11442793').subscribe((v) => (out = v));
    const req = http.expectOne(`${base}/targets/info?query=KIC%2011442793`);
    expect(req.request.method).toBe('GET');
    req.flush({ star: { sources: [] } });
    expect(out).toEqual({ star: { sources: [] } });
  });

  it('maps an error envelope and Retry-After', () => {
    let err: unknown;
    api.health().subscribe({ error: (e) => (err = e) });
    http
      .expectOne((r) => r.url.endsWith('/healthz'))
      .flush(
        { error: { code: 'busy', message: 'x' } },
        {
          status: 503,
          statusText: 'Service Unavailable',
          headers: { 'Retry-After': '7' },
        },
      );
    expect(err).toMatchObject({
      status: 503,
      code: 'busy',
      message: 'x',
      retryAfter: 7,
    });
    expect(err).toBeInstanceOf(ApiRequestError);
  });

  it('maps a network failure to unavailable', () => {
    let err: unknown;
    api.listRuns().subscribe({ error: (e) => (err = e) });
    http
      .expectOne((r) => r.url.endsWith('/ml/runs'))
      .error(new ProgressEvent('error'));
    expect(err).toMatchObject({ status: 0, code: 'unavailable' });
  });

  it('returns the 202 body of extract unchanged', () => {
    const queued = {
      status: 'queued',
      token: 't',
      subscription_path: '/subscription/t',
      position: 1,
    };
    let out: unknown;
    api.extract({ query: 'a' }).subscribe((v) => (out = v));
    http
      .expectOne((r) => r.url.endsWith('/pipeline/extract'))
      .flush(queued, { status: 202, statusText: 'Accepted' });
    expect(out).toEqual(queued);
  });

  describe('watch', () => {
    beforeEach(() => vi.stubGlobal('WebSocket', FakeSocket));

    it('opens the ws url and emits events per frame', () => {
      const frames: unknown[] = [];
      let done = false;
      api.watch('t', 4).subscribe({
        next: (e) => frames.push(e),
        complete: () => (done = true),
      });
      expect(FakeSocket.last.url).toBe(
        'ws://localhost:8080/subscription/t?since=4',
      );
      FakeSocket.last.onmessage?.({
        data: JSON.stringify({ events: [{ seq: 5 }] }),
      });
      expect(frames).toEqual([[{ seq: 5 }]]);
      FakeSocket.last.onclose?.({ code: 1000 });
      expect(done).toBe(true);
    });

    it('errors with the close code on 1013', () => {
      let err: unknown;
      api.watch('t').subscribe({ error: (e) => (err = e) });
      expect(FakeSocket.last.url).toBe('ws://localhost:8080/subscription/t');
      FakeSocket.last.onclose?.({ code: 1013 });
      expect(err).toEqual({ code: 1013 });
    });

    it('closes the socket on unsubscribe', () => {
      const sub = api.watch('t').subscribe();
      sub.unsubscribe();
      expect(FakeSocket.last.closed).toBe(true);
    });
  });
});
