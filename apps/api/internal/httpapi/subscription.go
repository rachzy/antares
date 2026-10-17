package httpapi

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/rachzy/antares/apps/api/internal/hub"
	"github.com/rachzy/antares/apps/api/internal/jobs"
	"github.com/rachzy/antares/apps/api/internal/service"
)

const keepAliveInterval = 20 * time.Second

type frame struct {
	Events []hub.Event `json:"events"`
}

func (a *API) subscription(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if !jobs.ValidToken(token) {
		a.writeError(w, r, &service.Error{Kind: service.KindNotFound, Message: "unknown or expired subscription"})
		return
	}
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		view, err := a.be.Status(r.Context(), token)
		if err != nil {
			a.writeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, view)
		return
	}

	var since uint64
	if raw := r.URL.Query().Get("since"); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			a.writeError(w, r, badRequest("since must be a non-negative integer"))
			return
		}
		since = parsed
	}
	watch, err := a.be.Watch(r.Context(), token, since)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	a.stream(w, r, watch)
}

func (a *API) stream(w http.ResponseWriter, r *http.Request, watch *service.Watch) {
	if watch.Sub != nil {
		defer watch.Sub.Cancel()
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: originHosts(a.cfg.AllowedOrigins)})
	if err != nil {
		return
	}
	defer func() { _ = conn.CloseNow() }()

	ctx := conn.CloseRead(r.Context())
	if watch.Sub == nil {
		events := watch.History
		if events == nil {
			events = []hub.Event{}
		}
		_ = wsjson.Write(ctx, conn, frame{Events: events})
		_ = conn.Close(websocket.StatusNormalClosure, "")
		return
	}

	go keepAlive(ctx, conn)
	for batch := range hub.Batches(ctx, watch.Sub, a.cfg.CoalesceInterval) {
		if err := wsjson.Write(ctx, conn, frame{Events: batch}); err != nil {
			return
		}
	}
	if watch.Sub.Dropped() {
		_ = conn.Close(websocket.StatusTryAgainLater, "slow consumer: reconnect with ?since=<last seq>")
		return
	}
	_ = conn.Close(websocket.StatusNormalClosure, "")
}

func keepAlive(ctx context.Context, conn *websocket.Conn) {
	ticker := time.NewTicker(keepAliveInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := conn.Ping(pingCtx)
			cancel()
			if err != nil {
				_ = conn.CloseNow()
				return
			}
		}
	}
}

func originHosts(origins []string) []string {
	hosts := make([]string, 0, len(origins))
	for _, origin := range origins {
		if u, err := url.Parse(origin); err == nil && u.Host != "" {
			hosts = append(hosts, u.Host)
			continue
		}
		hosts = append(hosts, origin)
	}
	return hosts
}
