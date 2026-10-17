package upstream

import (
	"context"
	"fmt"
	"io"
	"time"

	fangv1 "github.com/rachzy/antares/libs/proto/gen/go/antares/fang/v1"
	shaulav1 "github.com/rachzy/antares/libs/proto/gen/go/antares/shaula/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

// Dial returns a client connection to addr without connecting yet. It uses plaintext, as the
// services are private. The caller must Close it.
func Dial(addr string) (*grpc.ClientConn, error) {
	return grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
}

// mapError converts a gRPC error into a context error or an *Error. The caller's own context error
// comes first, so a caller that was cancelled or timed out gets context.Canceled or
// DeadlineExceeded, never an upstream fault. The deadline check covers a race: the deadline
// propagates to the server, which can fail with DeadlineExceeded before ctx.Err() is set here.
// Otherwise NotFound, InvalidArgument and FailedPrecondition keep their meaning; Unavailable, the
// server's own DeadlineExceeded and ResourceExhausted are retryable; anything else is internal.
func mapError(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if dl, ok := ctx.Deadline(); ok && status.Code(err) == codes.DeadlineExceeded && !time.Now().Before(dl) {
		return context.DeadlineExceeded
	}
	st, ok := status.FromError(err)
	if !ok {
		return &Error{Kind: KindInternal, Message: err.Error()}
	}
	kind := KindInternal
	switch st.Code() {
	case codes.NotFound:
		kind = KindNotFound
	case codes.InvalidArgument:
		kind = KindInvalid
	case codes.FailedPrecondition:
		kind = KindPrecondition
	case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted:
		kind = KindUnavailable
	}
	return &Error{Kind: kind, Message: st.Message()}
}

func toStructs(rows []map[string]any) ([]*structpb.Struct, error) {
	out := make([]*structpb.Struct, len(rows))
	for i, row := range rows {
		s, err := structpb.NewStruct(row)
		if err != nil {
			return nil, &Error{Kind: KindInvalid, Message: fmt.Sprintf("row %d cannot be sent: %v", i, err)}
		}
		out[i] = s
	}
	return out, nil
}

func fromStructs(in []*structpb.Struct) []map[string]any {
	out := make([]map[string]any, len(in))
	for i, s := range in {
		out[i] = s.AsMap()
	}
	return out
}

type shaulaClient struct{ c shaulav1.ShaulaClient }

// NewShaula returns a Shaula client over cc.
func NewShaula(cc grpc.ClientConnInterface) Shaula {
	return &shaulaClient{c: shaulav1.NewShaulaClient(cc)}
}

func (s *shaulaClient) Version(ctx context.Context) (string, error) {
	r, err := s.c.GetVersion(ctx, &shaulav1.GetVersionRequest{})
	if err != nil {
		return "", mapError(ctx, err)
	}
	return r.GetLibraryVersion(), nil
}

func (s *shaulaClient) Resolve(ctx context.Context, req ResolveRequest) (Resolved, error) {
	r, err := s.c.ResolveTarget(ctx, &shaulav1.ResolveTargetRequest{Query: req.Query, Mission: req.Mission, RaDeg: req.RA, DecDeg: req.Dec})
	if err != nil {
		return Resolved{}, mapError(ctx, err)
	}
	return Resolved{
		Catalog:     r.GetCatalog(),
		CatalogID:   r.GetCatalogId(),
		RA:          r.GetRaDeg(),
		Dec:         r.GetDecDeg(),
		DisplayName: r.GetDisplayName(),
		Designation: r.GetDesignation(),
	}, nil
}

func (s *shaulaClient) Extract(ctx context.Context, req ExtractRequest, onProgress func(Progress)) (FeatureSet, error) {
	stream, err := s.c.ExtractFeatures(ctx, &shaulav1.ExtractFeaturesRequest{
		Target:       req.Target,
		Mission:      req.Mission,
		SigmaClip:    req.SigmaClip,
		DownloadAll:  req.DownloadAll,
		Author:       req.Author,
		Exptime:      req.Exptime,
		UseTls:       req.UseTLS,
		MaskEclipses: req.MaskEclipses,
	})
	if err != nil {
		return FeatureSet{}, mapError(ctx, err)
	}

	var result *FeatureSet
	for {
		event, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return FeatureSet{}, mapError(ctx, err)
		}
		switch e := event.GetEvent().(type) {
		case *shaulav1.ExtractFeaturesEvent_Progress:
			p := Progress{Stage: e.Progress.GetStage(), Message: e.Progress.GetMessage()}
			if e.Progress.GetHasFraction() {
				fraction := e.Progress.GetFraction()
				p.Fraction = &fraction
			}
			onProgress(p)
		case *shaulav1.ExtractFeaturesEvent_Result:
			result = &FeatureSet{
				Target:         e.Result.GetTarget(),
				Mission:        e.Result.GetMission(),
				Rows:           fromStructs(e.Result.GetFeatures()),
				LibraryVersion: e.Result.GetLibraryVersion(),
			}
		}
	}
	if result == nil {
		return FeatureSet{}, &Error{Kind: KindInternal, Message: "the extraction stream ended without a result"}
	}
	return *result, nil
}

type fangClient struct{ c fangv1.FangClient }

// NewFang returns a Fang client over cc.
func NewFang(cc grpc.ClientConnInterface) Fang {
	return &fangClient{c: fangv1.NewFangClient(cc)}
}

func (f *fangClient) Version(ctx context.Context) (string, error) {
	r, err := f.c.GetVersion(ctx, &fangv1.GetVersionRequest{})
	if err != nil {
		return "", mapError(ctx, err)
	}
	return r.GetLibraryVersion(), nil
}

func (f *fangClient) ListRuns(ctx context.Context) ([]Run, error) {
	r, err := f.c.ListRuns(ctx, &fangv1.ListRunsRequest{})
	if err != nil {
		return nil, mapError(ctx, err)
	}
	runs := make([]Run, 0, len(r.GetRuns()))
	for _, run := range r.GetRuns() {
		runs = append(runs, Run{
			RunID:         run.GetRunId(),
			CreatedAt:     run.GetCreatedAt(),
			SchemaVersion: int(run.GetSchemaVersion()),
			SelectedModel: run.GetSelectedModel(),
			Threshold:     run.GetThreshold(),
		})
	}
	return runs, nil
}

func (f *fangClient) Predict(ctx context.Context, req PredictRequest) (Prediction, error) {
	rows, err := toStructs(req.Rows)
	if err != nil {
		return Prediction{}, err
	}
	r, err := f.c.Predict(ctx, &fangv1.PredictRequest{Features: rows, RunId: req.RunID, StarId: req.StarID})
	if err != nil {
		return Prediction{}, mapError(ctx, err)
	}
	return Prediction{
		Rows:          fromStructs(r.GetPredictions()),
		ModelRunID:    r.GetModelRunId(),
		SchemaVersion: int(r.GetSchemaVersion()),
	}, nil
}
