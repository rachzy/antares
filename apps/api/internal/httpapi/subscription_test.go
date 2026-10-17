package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/rachzy/antares/apps/api/internal/config"
	"github.com/rachzy/antares/apps/api/internal/httpapi"
	"github.com/rachzy/antares/apps/api/internal/hub"
	"github.com/rachzy/antares/apps/api/internal/jobs"
	"github.com/rachzy/antares/apps/api/internal/service"
)

func newToken(t *testing.T) string {
	t.Helper()
	token, err := jobs.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func serve(t *testing.T, s *stub, mutate func(*config.Config)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler(t, s, func(c *config.Config) {
		c.CoalesceInterval = 10 * time.Millisecond
		if mutate != nil {
			mutate(c)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func wsURL(srv *httptest.Server, path string) string {
	return "ws" + strings.TrimPrefix(srv.URL, "http") + path
}

func dialWS(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", url, err)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })
	return conn
}

func readEvents(conn *websocket.Conn) ([]hub.Event, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var frame struct {
		Events []hub.Event `json:"events"`
	}
	err := wsjson.Read(ctx, conn, &frame)
	return frame.Events, err
}

func drain(conn *websocket.Conn) (events []hub.Event, closeStatus websocket.StatusCode) {
	for {
		batch, err := readEvents(conn)
		if err != nil {
			return events, websocket.CloseStatus(err)
		}
		events = append(events, batch...)
	}
}

func TestSubscriptionStreamsReplayThenLiveEventsAndClosesNormally(t *testing.T) {
	room := hub.New(100, 16, time.Now).Room("job")
	room.Append(hub.State("queued", "Waiting"))
	room.Append(hub.State("running", "Started"))

	var sinceSeen uint64
	srv := serve(t, &stub{watch: func(_ context.Context, _ string, since uint64) (*service.Watch, error) {
		sinceSeen = since
		return &service.Watch{Sub: room.Subscribe(since)}, nil
	}}, nil)

	conn := dialWS(t, wsURL(srv, "/subscription/"+newToken(t)))
	first, err := readEvents(conn)
	if err != nil || len(first) != 2 || first[0].Seq != 1 || first[1].Seq != 2 {
		t.Fatalf("replay frame = %+v, err %v", first, err)
	}

	go func() {
		time.Sleep(20 * time.Millisecond)
		room.Append(hub.Progress("period_search", "Evaluating candidate 1", nil))
		room.Append(hub.Progress("period_search", "Evaluating candidate 2", nil))
		room.Append(hub.Result(json.RawMessage(`{"features":[]}`)))
		room.Close()
	}()

	rest, status := drain(conn)
	var seqs []uint64
	for _, ev := range rest {
		seqs = append(seqs, ev.Seq)
	}
	if len(seqs) != 3 || seqs[0] != 3 || seqs[2] != 5 {
		t.Fatalf("live events = %v, want seqs 3..5 with no gap", seqs)
	}
	if last := rest[len(rest)-1]; last.Type != hub.TypeResult || string(last.Payload) != `{"features":[]}` {
		t.Fatalf("the stream must end with the result, got %+v", last)
	}
	if status != websocket.StatusNormalClosure {
		t.Fatalf("close status = %d, want 1000", status)
	}
	if sinceSeen != 0 {
		t.Fatalf("since defaulted to %d", sinceSeen)
	}
}

func TestSinceIsPassedThroughAndOnlyNewerEventsAreReplayed(t *testing.T) {
	room := hub.New(100, 16, time.Now).Room("job")
	for i := 0; i < 4; i++ {
		room.Append(hub.State("s", "m"))
	}
	room.Close()

	var sinceSeen uint64
	srv := serve(t, &stub{watch: func(_ context.Context, _ string, since uint64) (*service.Watch, error) {
		sinceSeen = since
		return &service.Watch{Sub: room.Subscribe(since)}, nil
	}}, nil)

	conn := dialWS(t, wsURL(srv, "/subscription/"+newToken(t)+"?since=2"))
	events, status := drain(conn)
	if sinceSeen != 2 || len(events) != 2 || events[0].Seq != 3 || events[1].Seq != 4 {
		t.Fatalf("since=%d events=%+v", sinceSeen, events)
	}
	if status != websocket.StatusNormalClosure {
		t.Fatalf("close status = %d", status)
	}
}

func TestASlowConsumerIsToldToReconnect(t *testing.T) {
	room := hub.New(100, 1, time.Now).Room("job")
	sub := room.Subscribe(0)
	for i := 0; i < 5; i++ {
		room.Append(hub.State("s", "m"))
	}
	if !sub.Dropped() {
		t.Fatal("test setup: the subscription should have been dropped for being slow")
	}

	srv := serve(t, &stub{watch: func(context.Context, string, uint64) (*service.Watch, error) {
		return &service.Watch{Sub: sub}, nil
	}}, nil)
	conn := dialWS(t, wsURL(srv, "/subscription/"+newToken(t)))
	_, status := drain(conn)
	if status != websocket.StatusTryAgainLater {
		t.Fatalf("close status = %d, want 1013 (try again later)", status)
	}
}

func TestAFinishedJobIsServedFromHistoryThenClosed(t *testing.T) {
	history := []hub.Event{
		{Seq: 1, Type: hub.TypeState, Stage: "queued"},
		{Seq: 2, Type: hub.TypeState, Stage: "running"},
		{Seq: 3, Type: hub.TypeResult, Payload: json.RawMessage(`{"ok":true}`)},
	}
	srv := serve(t, &stub{watch: func(context.Context, string, uint64) (*service.Watch, error) {
		return &service.Watch{History: history}, nil
	}}, nil)

	conn := dialWS(t, wsURL(srv, "/subscription/"+newToken(t)))
	events, status := drain(conn)
	if len(events) != 3 || events[2].Type != hub.TypeResult || status != websocket.StatusNormalClosure {
		t.Fatalf("events = %+v status = %d", events, status)
	}
}

func TestAHistoryWithNothingNewStillClosesCleanly(t *testing.T) {
	srv := serve(t, &stub{watch: func(context.Context, string, uint64) (*service.Watch, error) {
		return &service.Watch{History: nil}, nil
	}}, nil)
	conn := dialWS(t, wsURL(srv, "/subscription/"+newToken(t)+"?since=9"))
	events, status := drain(conn)
	if len(events) != 0 || status != websocket.StatusNormalClosure {
		t.Fatalf("events = %+v status = %d", events, status)
	}
}

func TestUnknownMalformedAndBadSinceAreRejectedBeforeUpgrading(t *testing.T) {
	calls := 0
	h := handler(t, &stub{
		watch: func(context.Context, string, uint64) (*service.Watch, error) {
			calls++
			return nil, &service.Error{Kind: service.KindNotFound, Message: "unknown or expired subscription"}
		},
		status: func(context.Context, string) (service.SubscriptionView, error) {
			calls++
			return service.SubscriptionView{}, &service.Error{Kind: service.KindNotFound, Message: "unknown or expired subscription"}
		},
	}, nil)

	upgrade := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Upgrade", "websocket")
		req.Header.Set("Connection", "Upgrade")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	if rec := upgrade("/subscription/abc"); rec.Code != 404 || calls != 0 {
		t.Fatalf("a malformed token must be 404 without touching the backend: %d, calls=%d", rec.Code, calls)
	}
	if rec := do(h, "GET", "/subscription/abc", ""); rec.Code != 404 || calls != 0 {
		t.Fatalf("a malformed token on the status route: %d, calls=%d", rec.Code, calls)
	}
	valid := newToken(t)
	if rec := upgrade("/subscription/" + valid); rec.Code != 404 || decodeError(t, rec).Error.Code != "not_found" {
		t.Fatalf("unknown token: %d %s", rec.Code, rec.Body.String())
	}
	if rec := upgrade("/subscription/" + valid + "?since=abc"); rec.Code != 400 {
		t.Fatalf("since=abc: %d", rec.Code)
	}
	if rec := upgrade("/subscription/" + valid + "?since=-1"); rec.Code != 400 {
		t.Fatalf("since=-1: %d", rec.Code)
	}
}

func TestWithoutAnUpgradeTheEndpointReturnsAJSONStatus(t *testing.T) {
	var asked string
	h := handler(t, &stub{status: func(_ context.Context, token string) (service.SubscriptionView, error) {
		asked = token
		return service.SubscriptionView{
			Kind: jobs.KindAnalyze, State: jobs.StateFailed, Error: jobs.InterruptedMessage,
			Milestones: []service.MilestoneView{{Seq: 1, Stage: "queued"}}, LastSeq: 2,
		}, nil
	}}, nil)

	token := newToken(t)
	rec := do(h, "GET", "/subscription/"+token, "")
	if rec.Code != 200 || asked != token {
		t.Fatalf("status %d, asked %q", rec.Code, asked)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["state"] != "failed" || body["error"] != jobs.InterruptedMessage || body["last_seq"] != float64(2) || body["kind"] != "analyze" {
		t.Fatalf("body = %v", body)
	}
}

func TestADisallowedOriginIsRejected(t *testing.T) {
	srv := serve(t, &stub{watch: func(context.Context, string, uint64) (*service.Watch, error) {
		return &service.Watch{History: []hub.Event{}}, nil
	}}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, resp, err := websocket.Dial(ctx, wsURL(srv, "/subscription/"+newToken(t)), &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": {"https://evil.example"}},
	})
	if err == nil || resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("a foreign origin must get 403, got err=%v resp=%v", err, resp)
	}

	allowed := serve(t, &stub{watch: func(context.Context, string, uint64) (*service.Watch, error) {
		return &service.Watch{History: []hub.Event{}}, nil
	}}, func(c *config.Config) { c.AllowedOrigins = []string{"http://app.example"} })
	conn, _, err := websocket.Dial(ctx, wsURL(allowed, "/subscription/"+newToken(t)), &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": {"http://app.example"}},
	})
	if err != nil {
		t.Fatalf("a configured origin must be accepted: %v", err)
	}
	_ = conn.CloseNow()
}

func TestTheTokenNeverAppearsInTheLogs(t *testing.T) {
	var logs bytes.Buffer
	cfg, _ := config.Load(func(string) string { return "" })
	h := httpapi.New(&stub{status: func(context.Context, string) (service.SubscriptionView, error) {
		return service.SubscriptionView{State: jobs.StateDone}, nil
	}}, cfg, slog.New(slog.NewTextHandler(&logs, nil)))

	token := newToken(t)
	do(h, "GET", "/subscription/"+token, "")
	if strings.Contains(logs.String(), token) {
		t.Fatalf("the bearer token leaked into the request log:\n%s", logs.String())
	}
	if strings.Contains(logs.String(), "/subscription/"+token) {
		t.Fatalf("the URL path leaked into the request log:\n%s", logs.String())
	}
	if !strings.Contains(logs.String(), "route=\"GET /subscription/{token}\"") {
		t.Fatalf("the route pattern should be logged instead:\n%s", logs.String())
	}
}
