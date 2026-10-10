package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	mcpcenter "github.com/wanstu/mcp-center-core"
)

func TestScheduledProbeRunsOnEnableAndInterval(t *testing.T) {
	var requestCount atomic.Int64
	upstreamMCP := mcp.NewServer(&mcp.Implementation{Name: "scheduled-probe-upstream", Version: "1.0.0"}, nil)
	mcp.AddTool(upstreamMCP, &mcp.Tool{Name: "ping", Description: "ping"}, func(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "pong"}}}, nil, nil
	})
	upstreamHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return upstreamMCP }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		upstreamHandler.ServeHTTP(w, r)
	}))
	defer upstream.Close()

	app := newTestApp(t)
	def, err := app.core.Add(mcpcenter.Config{
		Name:      "scheduled-probe",
		Transport: mcpcenter.TransportStreamableHTTP,
		AuthMode:  mcpcenter.AuthNone,
		Endpoint:  upstream.URL,
		HealthPolicy: mcpcenter.HealthPolicy{
			HealthCheckEnabled:   true,
			CheckIntervalSeconds: 30,
			ProbeTimeoutSeconds:  2,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	now := time.Now().UTC()
	app.runScheduledProbePass(ctx, now)
	waitForScheduledProbe(t, app, def.ID, func(view scheduledProbeView) bool {
		return !view.Running && view.LastCheckedAt != nil && view.LastState == "healthy"
	})
	firstCount := requestCount.Load()
	if firstCount == 0 {
		t.Fatal("first scheduled probe did not reach upstream")
	}

	app.runScheduledProbePass(ctx, now.Add(10*time.Second))
	time.Sleep(150 * time.Millisecond)
	if got := requestCount.Load(); got != firstCount {
		t.Fatalf("probe ran before interval: requests=%d, want %d", got, firstCount)
	}

	app.runScheduledProbePass(ctx, now.Add(31*time.Second))
	deadline := time.Now().Add(3 * time.Second)
	for requestCount.Load() <= firstCount && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if requestCount.Load() <= firstCount {
		t.Fatal("second scheduled probe did not run after interval")
	}

	if _, err := app.core.SetDisabled(def.ID, true); err != nil {
		t.Fatal(err)
	}
	app.resetScheduledProbe(def.ID)
	app.runScheduledProbePass(ctx, now.Add(70*time.Second))
	time.Sleep(150 * time.Millisecond)
	views := app.scheduledProbeViews()
	if len(views) != 1 || views[0].MCPID != def.ID || views[0].Active {
		t.Fatalf("disabled scheduled probe view = %#v", views)
	}
}

func TestScheduledProbeDefaultsInterval(t *testing.T) {
	if got := scheduledProbeInterval(mcpcenter.HealthPolicy{}); got != 60*time.Second {
		t.Fatalf("default interval = %s", got)
	}
	if got := scheduledProbeInterval(mcpcenter.HealthPolicy{CheckIntervalSeconds: 15}); got != 15*time.Second {
		t.Fatalf("configured interval = %s", got)
	}
}

func waitForScheduledProbe(t *testing.T, app *App, id string, ready func(scheduledProbeView) bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		for _, view := range app.scheduledProbeViews() {
			if view.MCPID == id && ready(view) {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("scheduled probe %s did not reach expected state: %#v", id, app.scheduledProbeViews())
}
