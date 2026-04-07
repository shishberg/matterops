package bot

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
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

// --- Mocks ---

// mockWSClient implements wsClient with controllable channels.
type mockWSClient struct {
	events      chan *model.WebSocketEvent
	pingTimeout chan bool
	listenErr   *model.AppError
	mu          sync.Mutex
	closed      bool
}

func newMockWSClient() *mockWSClient {
	return &mockWSClient{
		events:      make(chan *model.WebSocketEvent, 10),
		pingTimeout: make(chan bool, 1),
	}
}

func (m *mockWSClient) Listen() {}

func (m *mockWSClient) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.closed {
		m.closed = true
		close(m.events)
	}
}

func (m *mockWSClient) EventChan() chan *model.WebSocketEvent { return m.events }
func (m *mockWSClient) PingTimeoutChan() chan bool            { return m.pingTimeout }

func (m *mockWSClient) GetListenError() *model.AppError {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.listenErr
}

func (m *mockWSClient) SetListenError(err *model.AppError) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listenErr = err
}

// mockRestClient implements restClient for testing.
type mockRestClient struct {
	mu    sync.Mutex
	posts []*model.Post
}

func (m *mockRestClient) GetMe(_ context.Context, _ string) (*model.User, *model.Response, error) {
	return &model.User{Id: "bot-user-id"}, &model.Response{}, nil
}

func (m *mockRestClient) CreatePost(_ context.Context, post *model.Post) (*model.Post, *model.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.posts = append(m.posts, post)
	return post, &model.Response{}, nil
}

func (m *mockRestClient) GetChannel(_ context.Context, channelId string) (*model.Channel, *model.Response, error) {
	return &model.Channel{Id: channelId}, &model.Response{}, nil
}

func (m *mockRestClient) GetChannelByNameForTeamName(_ context.Context, channelName, teamName string, _ string) (*model.Channel, *model.Response, error) {
	return &model.Channel{Id: "channel-id-123"}, &model.Response{}, nil
}

func (m *mockRestClient) getPosts() []*model.Post {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*model.Post, len(m.posts))
	copy(result, m.posts)
	return result
}

// mockCommandHandler implements CommandHandler for testing.
type mockCommandHandler struct {
	lastAction  string
	lastService string
	mu          sync.Mutex
}

func (h *mockCommandHandler) HandleStatus() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastAction = "status"
	return "status response"
}

func (h *mockCommandHandler) HandleDeploy(svc string) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastAction = "deploy"
	h.lastService = svc
	return "deploy response"
}

func (h *mockCommandHandler) HandleRestart(svc string) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastAction = "restart"
	h.lastService = svc
	return "restart response"
}

func (h *mockCommandHandler) HandleConfirm(svc string) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastAction = "confirm"
	h.lastService = svc
	return "confirm response"
}

func (h *mockCommandHandler) getLastAction() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lastAction
}

// --- Reconnection tests ---

func TestConnectAndListen_GracefulShutdown(t *testing.T) {
	ws := newMockWSClient()
	bot := &Bot{
		cfg: Config{URL: "http://localhost"},
		connectWS: func(url, token string) (wsClient, error) {
			return ws, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- bot.connectAndListen(ctx)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("connectAndListen did not return after cancel")
	}
}

func TestConnectAndListen_PingTimeout(t *testing.T) {
	ws := newMockWSClient()
	bot := &Bot{
		cfg: Config{URL: "http://localhost"},
		connectWS: func(url, token string) (wsClient, error) {
			return ws, nil
		},
	}

	done := make(chan error, 1)
	go func() {
		done <- bot.connectAndListen(context.Background())
	}()

	// Simulate ping timeout
	ws.PingTimeoutChan() <- true

	select {
	case err := <-done:
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ping timeout")
	case <-time.After(2 * time.Second):
		t.Fatal("connectAndListen did not return on ping timeout")
	}
}

func TestConnectAndListen_ListenError(t *testing.T) {
	ws := newMockWSClient()
	bot := &Bot{
		cfg: Config{URL: "http://localhost"},
		connectWS: func(url, token string) (wsClient, error) {
			return ws, nil
		},
	}

	done := make(chan error, 1)
	go func() {
		done <- bot.connectAndListen(context.Background())
	}()

	// Set a listen error then send an event to trigger the error check
	ws.SetListenError(model.NewAppError("ws", "connection lost", nil, "", 0))
	ws.EventChan() <- nil

	select {
	case err := <-done:
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "listen error")
	case <-time.After(2 * time.Second):
		t.Fatal("connectAndListen did not return on listen error")
	}
}

func TestListenWebSocket_ReconnectsAfterDisconnect(t *testing.T) {
	var connectCount atomic.Int32

	bot := &Bot{
		cfg: Config{URL: "http://localhost"},
		connectWS: func(url, token string) (wsClient, error) {
			connectCount.Add(1)
			ws := newMockWSClient()
			// Simulate immediate ping timeout to force reconnection
			go func() {
				ws.PingTimeoutChan() <- true
			}()
			return ws, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- bot.listenWebSocket(ctx)
	}()

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
}

func TestListenWebSocket_CancelDuringBackoff(t *testing.T) {
	bot := &Bot{
		cfg: Config{URL: "http://localhost"},
		connectWS: func(url, token string) (wsClient, error) {
			ws := newMockWSClient()
			go func() { ws.PingTimeoutChan() <- true }()
			return ws, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- bot.listenWebSocket(ctx)
	}()

	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("listenWebSocket did not exit during backoff")
	}
}

func TestListenWebSocket_ConnectFailureReconnects(t *testing.T) {
	var attempts atomic.Int32

	bot := &Bot{
		cfg: Config{URL: "http://localhost"},
		connectWS: func(url, token string) (wsClient, error) {
			attempts.Add(1)
			return nil, fmt.Errorf("connection refused")
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- bot.listenWebSocket(ctx)
	}()

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

// --- Event handling tests ---

func TestHandleEvent_DispatchesCommand(t *testing.T) {
	handler := &mockCommandHandler{}
	rest := &mockRestClient{}

	bot := &Bot{
		cfg:       Config{URL: "http://localhost", Handler: handler},
		rest:      rest,
		userID:    "bot-user-id",
		channelID: "channel-123",
	}

	postJSON := `{"id":"post1","user_id":"other-user","channel_id":"channel-123","message":"@matterops deploy myapp"}`
	event := model.NewWebSocketEvent(model.WebsocketEventPosted, "", "channel-123", "", nil, "")
	event = event.SetData(map[string]any{
		"channel_id": "channel-123",
		"post":       postJSON,
	})

	bot.handleEvent(context.Background(), event)

	assert.Equal(t, "deploy", handler.getLastAction())
	posts := rest.getPosts()
	require.Len(t, posts, 1)
	assert.Equal(t, "deploy response", posts[0].Message)
}

func TestHandleEvent_IgnoresOwnMessages(t *testing.T) {
	handler := &mockCommandHandler{}
	rest := &mockRestClient{}

	bot := &Bot{
		cfg:       Config{URL: "http://localhost", Handler: handler},
		rest:      rest,
		userID:    "bot-user-id",
		channelID: "channel-123",
	}

	postJSON := `{"id":"post1","user_id":"bot-user-id","channel_id":"channel-123","message":"@matterops status"}`
	event := model.NewWebSocketEvent(model.WebsocketEventPosted, "", "channel-123", "", nil, "")
	event = event.SetData(map[string]any{
		"channel_id": "channel-123",
		"post":       postJSON,
	})

	bot.handleEvent(context.Background(), event)

	assert.Empty(t, handler.getLastAction())
	assert.Empty(t, rest.getPosts())
}

func TestHandleEvent_IgnoresOtherChannels(t *testing.T) {
	handler := &mockCommandHandler{}
	rest := &mockRestClient{}

	bot := &Bot{
		cfg:       Config{URL: "http://localhost", Handler: handler},
		rest:      rest,
		userID:    "bot-user-id",
		channelID: "channel-123",
	}

	postJSON := `{"id":"post1","user_id":"other-user","channel_id":"other-channel","message":"@matterops status"}`
	event := model.NewWebSocketEvent(model.WebsocketEventPosted, "", "other-channel", "", nil, "")
	event = event.SetData(map[string]any{
		"channel_id": "other-channel",
		"post":       postJSON,
	})

	bot.handleEvent(context.Background(), event)

	assert.Empty(t, handler.getLastAction())
	assert.Empty(t, rest.getPosts())
}

func TestHandleEvent_IgnoresNonPostedEvents(t *testing.T) {
	handler := &mockCommandHandler{}

	bot := &Bot{
		cfg:       Config{URL: "http://localhost", Handler: handler},
		userID:    "bot-user-id",
		channelID: "channel-123",
	}

	event := model.NewWebSocketEvent(model.WebsocketEventTyping, "", "channel-123", "", nil, "")

	bot.handleEvent(context.Background(), event)

	assert.Empty(t, handler.getLastAction())
}

// --- REST integration tests ---

func TestResolveIdentity(t *testing.T) {
	rest := &mockRestClient{}
	bot := &Bot{rest: rest}

	err := bot.resolveIdentity(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "bot-user-id", bot.userID)
}

func TestResolveChannel_TeamSlashChannel(t *testing.T) {
	rest := &mockRestClient{}
	bot := &Bot{
		cfg:  Config{Channel: "myteam/ops"},
		rest: rest,
	}

	err := bot.resolveChannel(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "channel-id-123", bot.channelID)
}

func TestResolveChannel_RawID(t *testing.T) {
	rest := &mockRestClient{}
	bot := &Bot{
		cfg:  Config{Channel: "abcdefghijklmnopqrstuvwxyz"}, // 26-char lowercase
		rest: rest,
	}

	err := bot.resolveChannel(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "abcdefghijklmnopqrstuvwxyz", bot.channelID)
}

func TestResolveChannel_InvalidFormat(t *testing.T) {
	bot := &Bot{
		cfg: Config{Channel: "⚙️ Ops"},
	}

	err := bot.resolveChannel(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "team/channel-name")
}
