package bot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    *Command
	}{
		{"status command", "@matterops status", &Command{Action: "status"}},
		{"deploy command", "@matterops deploy myapp", &Command{Action: "deploy", Service: "myapp"}},
		{"restart command", "@matterops restart myapp", &Command{Action: "restart", Service: "myapp"}},
		{"confirm command", "@matterops confirm myapp", &Command{Action: "confirm", Service: "myapp"}},
		{"with extra whitespace", "  @matterops   deploy   myapp  ", &Command{Action: "deploy", Service: "myapp"}},
		{"not a command", "hello world", nil},
		{"empty after mention", "@matterops", nil},
		{"unknown command", "@matterops foobar", &Command{Action: "foobar"}},
		{"case insensitive mention", "@MatterOps deploy myapp", &Command{Action: "deploy", Service: "myapp"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseCommand(tt.message)
			assert.Equal(t, tt.want, got)
		})
	}
}

// wsTestServer creates an httptest server that upgrades to WebSocket, calls onConn,
// and returns the server URL (http://...).
func wsTestServer(t *testing.T, onConn func(*websocket.Conn)) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}
		defer func() { _ = conn.Close() }()
		onConn(conn)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// testDial returns a dialFunc that connects to the given test server URL,
// ignoring the url argument (so the bot's URL construction is bypassed).
func testDial(serverURL string) dialFunc {
	wsURL := "ws" + strings.TrimPrefix(serverURL, "http")
	return func(ctx context.Context, _ string, header http.Header) (*websocket.Conn, error) {
		dialer := websocket.Dialer{HandshakeTimeout: 2 * time.Second}
		conn, _, err := dialer.DialContext(ctx, wsURL, header)
		return conn, err
	}
}

func TestConnectAndListen_GracefulShutdown(t *testing.T) {
	srv := wsTestServer(t, func(conn *websocket.Conn) {
		// Keep connection open until client disconnects
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	})

	bot := &Bot{
		cfg:  Config{URL: srv.URL},
		dial: testDial(srv.URL),
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- bot.connectAndListen(ctx)
	}()

	// Give it time to connect
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err) // graceful shutdown returns nil
	case <-time.After(2 * time.Second):
		t.Fatal("connectAndListen did not return after cancel")
	}
}

func TestConnectAndListen_ServerCloses(t *testing.T) {
	srv := wsTestServer(t, func(conn *websocket.Conn) {
		// Close immediately to simulate server disconnect
		_ = conn.Close()
	})

	bot := &Bot{
		cfg:  Config{URL: srv.URL},
		dial: testDial(srv.URL),
	}

	err := bot.connectAndListen(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "websocket closed unexpectedly")
}

func TestListenWebSocket_ReconnectsAfterDisconnect(t *testing.T) {
	var connectCount atomic.Int32

	srv := wsTestServer(t, func(conn *websocket.Conn) {
		connectCount.Add(1)
		// Close immediately — forces reconnection
		_ = conn.Close()
	})

	bot := &Bot{
		cfg:  Config{URL: srv.URL},
		dial: testDial(srv.URL),
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- bot.listenWebSocket(ctx)
	}()

	// Wait for at least 2 reconnections (initial connect + 1 reconnect after 1s backoff)
	require.Eventually(t, func() bool {
		return connectCount.Load() >= 2
	}, 5*time.Second, 100*time.Millisecond, "expected at least 2 connection attempts")

	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("listenWebSocket did not exit after cancel")
	}

	assert.GreaterOrEqual(t, connectCount.Load(), int32(2))
}

func TestListenWebSocket_CancelDuringBackoff(t *testing.T) {
	srv := wsTestServer(t, func(conn *websocket.Conn) {
		// Close immediately to trigger reconnect backoff
		_ = conn.Close()
	})

	bot := &Bot{
		cfg:  Config{URL: srv.URL},
		dial: testDial(srv.URL),
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- bot.listenWebSocket(ctx)
	}()

	// Wait for at least one connection attempt, then cancel during backoff
	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err) // should exit cleanly
	case <-time.After(2 * time.Second):
		t.Fatal("listenWebSocket did not exit during backoff")
	}
}

func TestListenWebSocket_DialFailureReconnects(t *testing.T) {
	var attempts atomic.Int32

	bot := &Bot{
		cfg: Config{URL: "http://localhost:1"},
		dial: func(ctx context.Context, url string, header http.Header) (*websocket.Conn, error) {
			attempts.Add(1)
			// Simulate connection refused
			dialer := websocket.Dialer{HandshakeTimeout: 100 * time.Millisecond}
			conn, _, err := dialer.DialContext(ctx, "ws://localhost:1/nope", header)
			return conn, err
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- bot.listenWebSocket(ctx)
	}()

	// Should retry at least once after the initial failure
	require.Eventually(t, func() bool {
		return attempts.Load() >= 2
	}, 5*time.Second, 100*time.Millisecond)

	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("listenWebSocket did not exit after cancel")
	}
}
