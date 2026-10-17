package upstream_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	fangv1 "github.com/rachzy/antares/libs/proto/gen/go/antares/fang/v1"
	shaulav1 "github.com/rachzy/antares/libs/proto/gen/go/antares/shaula/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/structpb"

	"antares/api/internal/upstream"
)

func dial(t *testing.T, register func(*grpc.Server)) *grpc.ClientConn {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	register(srv)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

type fakeShaulaServer struct {
	shaulav1.UnimplementedShaulaServer
	resolveSeen chan *shaulav1.ResolveTargetRequest
	extract     func(*shaulav1.ExtractFeaturesRequest, grpc.ServerStreamingServer[shaulav1.ExtractFeaturesEvent]) error
}

func (f *fakeShaulaServer) GetVersion(context.Context, *shaulav1.GetVersionRequest) (*shaulav1.GetVersionResponse, error) {
	return &shaulav1.GetVersionResponse{LibraryVersion: "0.0.1a0", ServiceVersion: "0.0.1a0"}, nil
}

func (f *fakeShaulaServer) ResolveTarget(_ context.Context, r *shaulav1.ResolveTargetRequest) (*shaulav1.ResolveTargetResponse, error) {
	if f.resolveSeen != nil {
		f.resolveSeen <- r
	}
	switch r.GetQuery() {
	case "notfound":
		return nil, status.Error(codes.NotFound, `No KIC entry found for "notfound"`)
	case "invalid":
		return nil, status.Error(codes.InvalidArgument, "Unsupported mission 'Hubble'")
	case "precondition":
		return nil, status.Error(codes.FailedPrecondition, "no light curves")
	case "down":
		return nil, status.Error(codes.Unavailable, "archive unreachable")
	case "deadline":
		return nil, status.Error(codes.DeadlineExceeded, "proxy timed out")
	case "boom":
		return nil, status.Error(codes.Internal, "kaboom")
	case "weird":
		return nil, status.Error(codes.Unknown, "who knows")
	}
	return &shaulav1.ResolveTargetResponse{
		Catalog: "KIC", CatalogId: "8120608", RaDeg: 298.65272, DecDeg: 43.95502,
		DisplayName: "Kepler-186", Designation: "KIC 8120608",
	}, nil
}

func (f *fakeShaulaServer) ExtractFeatures(r *shaulav1.ExtractFeaturesRequest, s grpc.ServerStreamingServer[shaulav1.ExtractFeaturesEvent]) error {
	return f.extract(r, s)
}

func progress(stage, message string, fraction *float64) *shaulav1.ExtractFeaturesEvent {
	p := &shaulav1.Progress{Stage: stage, Message: message}
	if fraction != nil {
		p.HasFraction, p.Fraction = true, *fraction
	}
	return &shaulav1.ExtractFeaturesEvent{Event: &shaulav1.ExtractFeaturesEvent_Progress{Progress: p}}
}

func shaulaClient(t *testing.T, extract func(*shaulav1.ExtractFeaturesRequest, grpc.ServerStreamingServer[shaulav1.ExtractFeaturesEvent]) error) upstream.Shaula {
	conn := dial(t, func(s *grpc.Server) {
		shaulav1.RegisterShaulaServer(s, &fakeShaulaServer{extract: extract})
	})
	return upstream.NewShaula(conn)
}

func TestShaulaVersionAndResolve(t *testing.T) {
	client := shaulaClient(t, nil)
	ctx := context.Background()

	version, err := client.Version(ctx)
	if err != nil || version != "0.0.1a0" {
		t.Fatalf("Version = %q, %v", version, err)
	}
	got, err := client.Resolve(ctx, upstream.ResolveRequest{Query: "Kepler-186f", Mission: "Kepler"})
	if err != nil {
		t.Fatal(err)
	}
	want := upstream.Resolved{
		Catalog: "KIC", CatalogID: "8120608", RA: 298.65272, Dec: 43.95502,
		DisplayName: "Kepler-186", Designation: "KIC 8120608",
	}
	if got != want {
		t.Fatalf("Resolve = %+v, want %+v", got, want)
	}
}

func TestResolveForwardsCoordinatesAndNames(t *testing.T) {
	seen := make(chan *shaulav1.ResolveTargetRequest, 4)
	conn := dial(t, func(s *grpc.Server) {
		shaulav1.RegisterShaulaServer(s, &fakeShaulaServer{resolveSeen: seen})
	})
	client := upstream.NewShaula(conn)
	ctx := context.Background()

	ra, dec := 0.0, 0.0
	if _, err := client.Resolve(ctx, upstream.ResolveRequest{RA: &ra, Dec: &dec, Mission: "Kepler"}); err != nil {
		t.Fatal(err)
	}
	got := <-seen
	if got.RaDeg == nil || got.DecDeg == nil || *got.RaDeg != 0 || *got.DecDeg != 0 {
		t.Fatalf("zero coordinates must arrive as set pointers to 0, got %v %v", got.RaDeg, got.DecDeg)
	}
	if got.GetQuery() != "" || got.GetMission() != "Kepler" {
		t.Errorf("unexpected request %+v", got)
	}

	ra, dec = 298.65272, -43.95502
	if _, err := client.Resolve(ctx, upstream.ResolveRequest{RA: &ra, Dec: &dec}); err != nil {
		t.Fatal(err)
	}
	got = <-seen
	if got.RaDeg == nil || got.DecDeg == nil || *got.RaDeg != 298.65272 || *got.DecDeg != -43.95502 {
		t.Fatalf("coordinates did not arrive intact: %v %v", got.RaDeg, got.DecDeg)
	}

	if _, err := client.Resolve(ctx, upstream.ResolveRequest{Query: "Kepler-186f", Mission: "Kepler"}); err != nil {
		t.Fatal(err)
	}
	got = <-seen
	if got.RaDeg != nil || got.DecDeg != nil || got.GetQuery() != "Kepler-186f" {
		t.Fatalf("a name request must carry no coordinates, got %+v", got)
	}
}

func TestShaulaErrorsKeepTheirKindAndMessage(t *testing.T) {
	client := shaulaClient(t, nil)
	for query, want := range map[string]upstream.Kind{
		"notfound":     upstream.KindNotFound,
		"invalid":      upstream.KindInvalid,
		"precondition": upstream.KindPrecondition,
		"down":         upstream.KindUnavailable,
		"boom":         upstream.KindInternal,
		"weird":        upstream.KindInternal,
	} {
		_, err := client.Resolve(context.Background(), upstream.ResolveRequest{Query: query, Mission: "Kepler"})
		if err == nil {
			t.Errorf("%s: expected an error", query)
			continue
		}
		if got := upstream.KindOf(err); got != want {
			t.Errorf("%s: kind = %v, want %v (%v)", query, got, want, err)
		}
	}
	_, err := client.Resolve(context.Background(), upstream.ResolveRequest{Query: "notfound", Mission: "Kepler"})
	if err == nil || err.Error() != `No KIC entry found for "notfound"` {
		t.Fatalf("the upstream message must be kept verbatim, got %v", err)
	}
}

func TestExtractRelaysProgressThenReturnsTheFeatures(t *testing.T) {
	half := 0.5
	var seen *shaulav1.ExtractFeaturesRequest
	client := shaulaClient(t, func(r *shaulav1.ExtractFeaturesRequest, s grpc.ServerStreamingServer[shaulav1.ExtractFeaturesEvent]) error {
		seen = r
		_ = s.Send(progress("downloading", "Downloading", nil))
		_ = s.Send(progress("period_search", "Evaluating candidate 1", &half))
		row, _ := structpb.NewStruct(map[string]any{"period_days": 129.9, "vshape_metric": nil, "detection_status": "accepted"})
		return s.Send(&shaulav1.ExtractFeaturesEvent{Event: &shaulav1.ExtractFeaturesEvent_Result{
			Result: &shaulav1.FeatureSet{Target: "KIC 8120608", Mission: "Kepler", Features: []*structpb.Struct{row}, LibraryVersion: "0.0.1a0"},
		}})
	})

	var events []upstream.Progress
	set, err := client.Extract(context.Background(), upstream.ExtractRequest{
		Target: "KIC 8120608", Mission: "Kepler", SigmaClip: 5, DownloadAll: true,
		Author: "Kepler", Exptime: "long", UseTLS: true, MaskEclipses: true,
	}, func(p upstream.Progress) { events = append(events, p) })
	if err != nil {
		t.Fatal(err)
	}

	if len(events) != 2 || events[0].Stage != "downloading" || events[0].Fraction != nil {
		t.Fatalf("events = %+v", events)
	}
	if events[1].Fraction == nil || *events[1].Fraction != 0.5 {
		t.Fatalf("a reported fraction must survive, a missing one must stay nil: %+v", events[1])
	}
	if set.LibraryVersion != "0.0.1a0" || len(set.Rows) != 1 {
		t.Fatalf("set = %+v", set)
	}
	row := set.Rows[0]
	if row["period_days"] != 129.9 || row["vshape_metric"] != nil || row["detection_status"] != "accepted" {
		t.Fatalf("row = %v", row)
	}
	if seen.GetTarget() != "KIC 8120608" || !seen.GetDownloadAll() || !seen.GetUseTls() ||
		!seen.GetMaskEclipses() || seen.GetSigmaClip() != 5 || seen.GetAuthor() != "Kepler" || seen.GetExptime() != "long" {
		t.Fatalf("request not forwarded intact: %+v", seen)
	}
}

func TestExtractErrorAfterProgressKeepsTheKindAndTheEarlierEvents(t *testing.T) {
	client := shaulaClient(t, func(_ *shaulav1.ExtractFeaturesRequest, s grpc.ServerStreamingServer[shaulav1.ExtractFeaturesEvent]) error {
		_ = s.Send(progress("downloading", "Downloading", nil))
		return status.Error(codes.FailedPrecondition, "No Kepler light curves found for 'x'")
	})
	calls := 0
	_, err := client.Extract(context.Background(), upstream.ExtractRequest{Target: "x"}, func(upstream.Progress) { calls++ })
	if upstream.KindOf(err) != upstream.KindPrecondition || err == nil || err.Error() != "No Kepler light curves found for 'x'" {
		t.Fatalf("err = %v (kind %v)", err, upstream.KindOf(err))
	}
	if calls != 1 {
		t.Fatalf("progress callbacks = %d, want the 1 sent before the failure", calls)
	}
}

func TestExtractStreamWithoutAResultIsAnError(t *testing.T) {
	client := shaulaClient(t, func(_ *shaulav1.ExtractFeaturesRequest, s grpc.ServerStreamingServer[shaulav1.ExtractFeaturesEvent]) error {
		return s.Send(progress("downloading", "", nil))
	})
	_, err := client.Extract(context.Background(), upstream.ExtractRequest{Target: "x"}, func(upstream.Progress) {})
	if upstream.KindOf(err) != upstream.KindInternal || err == nil {
		t.Fatalf("a stream that ends without a result must not look like success: %v", err)
	}
}

func TestExtractReturnsTheContextErrorWhenTheCallerGivesUp(t *testing.T) {
	client := shaulaClient(t, func(_ *shaulav1.ExtractFeaturesRequest, s grpc.ServerStreamingServer[shaulav1.ExtractFeaturesEvent]) error {
		<-s.Context().Done()
		return s.Context().Err()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := client.Extract(ctx, upstream.ExtractRequest{Target: "x"}, func(upstream.Progress) {})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want the context's own deadline error", err)
	}
}

type fakeFangServer struct {
	fangv1.UnimplementedFangServer
	lastPredict *fangv1.PredictRequest
}

func (f *fakeFangServer) GetVersion(context.Context, *fangv1.GetVersionRequest) (*fangv1.GetVersionResponse, error) {
	return &fangv1.GetVersionResponse{LibraryVersion: "0.0.1a0"}, nil
}

func (f *fakeFangServer) ListRuns(context.Context, *fangv1.ListRunsRequest) (*fangv1.ListRunsResponse, error) {
	return &fangv1.ListRunsResponse{Runs: []*fangv1.RunInfo{
		{RunId: "20261001T120000Z-abcd1234", CreatedAt: "2026-10-01T12:00:00Z", SchemaVersion: 1, SelectedModel: "lightgbm", Threshold: 0.42},
	}}, nil
}

func (f *fakeFangServer) Predict(_ context.Context, r *fangv1.PredictRequest) (*fangv1.PredictResponse, error) {
	f.lastPredict = r
	switch r.GetRunId() {
	case "missing":
		return nil, status.Error(codes.NotFound, `No servable run "missing"`)
	case "bad-rows":
		return nil, status.Error(codes.InvalidArgument, "missing required feature columns ['period_days']")
	case "wrong-schema":
		return nil, status.Error(codes.FailedPrecondition, "model expects schema version 2")
	}
	out := make([]*structpb.Struct, len(r.GetFeatures()))
	for i, row := range r.GetFeatures() {
		m := row.AsMap()
		m["prob_stack"] = 0.71
		s, _ := structpb.NewStruct(m)
		out[i] = s
	}
	return &fangv1.PredictResponse{Predictions: out, ModelRunId: r.GetRunId(), SchemaVersion: 1}, nil
}

func fangClient(t *testing.T) (upstream.Fang, *fakeFangServer) {
	server := &fakeFangServer{}
	conn := dial(t, func(s *grpc.Server) { fangv1.RegisterFangServer(s, server) })
	return upstream.NewFang(conn), server
}

func TestFangVersionRunsAndPredict(t *testing.T) {
	client, server := fangClient(t)
	ctx := context.Background()

	if v, err := client.Version(ctx); err != nil || v != "0.0.1a0" {
		t.Fatalf("Version = %q, %v", v, err)
	}
	runs, err := client.ListRuns(ctx)
	if err != nil || len(runs) != 1 {
		t.Fatalf("ListRuns = %+v, %v", runs, err)
	}
	if runs[0].RunID != "20261001T120000Z-abcd1234" || runs[0].SchemaVersion != 1 || runs[0].Threshold != 0.42 || runs[0].SelectedModel != "lightgbm" {
		t.Fatalf("run = %+v", runs[0])
	}

	got, err := client.Predict(ctx, upstream.PredictRequest{
		Rows:  []map[string]any{{"period_days": 3.5, "vshape_metric": nil}},
		RunID: "20261001T120000Z-abcd1234", StarID: "KIC-8120608",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ModelRunID != "20261001T120000Z-abcd1234" || got.SchemaVersion != 1 || got.Rows[0]["prob_stack"] != 0.71 {
		t.Fatalf("prediction = %+v", got)
	}
	if server.lastPredict.GetStarId() != "KIC-8120608" || server.lastPredict.GetRunId() != "20261001T120000Z-abcd1234" {
		t.Fatalf("request not forwarded: %+v", server.lastPredict)
	}
	if v := server.lastPredict.GetFeatures()[0].AsMap()["vshape_metric"]; v != nil {
		t.Fatalf("a missing value must reach Fang as null, got %v", v)
	}
}

func TestFangPredictErrorsKeepTheirKind(t *testing.T) {
	client, _ := fangClient(t)
	for run, want := range map[string]upstream.Kind{
		"missing":      upstream.KindNotFound,
		"bad-rows":     upstream.KindInvalid,
		"wrong-schema": upstream.KindPrecondition,
	} {
		_, err := client.Predict(context.Background(), upstream.PredictRequest{Rows: []map[string]any{{"a": 1.0}}, RunID: run})
		if upstream.KindOf(err) != want {
			t.Errorf("%s: kind = %v, want %v (%v)", run, upstream.KindOf(err), want, err)
		}
	}
}

func TestAnUnreachableServiceIsUnavailable(t *testing.T) {
	conn, err := grpc.NewClient("passthrough:///nowhere",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return nil, errors.New("connection refused") }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = upstream.NewShaula(conn).Version(ctx)
	if upstream.KindOf(err) != upstream.KindUnavailable {
		t.Fatalf("err = %v (kind %v), want KindUnavailable", err, upstream.KindOf(err))
	}
}

func TestAnUpstreamDeadlineWithinTheCallersDeadlineIsUnavailable(t *testing.T) {
	client := shaulaClient(t, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	_, err := client.Resolve(ctx, upstream.ResolveRequest{Query: "deadline", Mission: "Kepler"})
	if upstream.KindOf(err) != upstream.KindUnavailable {
		t.Fatalf("err = %v (kind %v), want KindUnavailable", err, upstream.KindOf(err))
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("returned after %v, must not wait for the caller's deadline", elapsed)
	}
}
