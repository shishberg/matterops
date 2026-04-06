package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// Command represents a parsed bot command.
type Command struct {
	Action  string
	Service string
}

// ParseCommand extracts a Command from a message that starts with @matterops.
// Returns nil if the message is not a valid command.
func ParseCommand(message string) *Command {
	fields := strings.Fields(message)
	if len(fields) == 0 {
		return nil
	}

	if !strings.EqualFold(fields[0], "@matterops") {
		return nil
	}

	if len(fields) < 2 {
		return nil
	}

	cmd := &Command{
		Action: fields[1],
	}
	if len(fields) >= 3 {
		cmd.Service = fields[2]
	}
	return cmd
}

// CommandHandler dispatches parsed bot commands to business logic.
type CommandHandler interface {
	HandleStatus() string
	HandleDeploy(service string) string
	HandleRestart(service string) string
	HandleConfirm(service string) string
}

// Config holds all configuration needed to connect to Mattermost.
type Config struct {
	URL     string
	Token   string
	Channel string
	Handler CommandHandler
}

// Bot connects to Mattermost via REST and WebSocket and dispatches commands.
type Bot struct {
	cfg       Config
	userID    string
	channelID string
	client    *http.Client
}

// New creates a new Bot from the given Config.
func New(cfg Config) *Bot {
	return &Bot{
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Run connects to Mattermost, resolves the channel, and listens for events
// until ctx is cancelled.
func (b *Bot) Run(ctx context.Context) error {
	if err := b.resolveIdentity(ctx); err != nil {
		return fmt.Errorf("resolving bot identity: %w", err)
	}

	if err := b.resolveChannel(ctx); err != nil {
		return fmt.Errorf("resolving channel: %w", err)
	}

	return b.listenWebSocket(ctx)
}

// PostMessage sends a message to the configured Mattermost channel.
func (b *Bot) PostMessage(ctx context.Context, message string) {
	body := fmt.Sprintf(`{"channel_id":%q,"message":%q}`, b.channelID, message)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		b.cfg.URL+"/api/v4/posts", strings.NewReader(body))
	if err != nil {
		log.Printf("bot: create post request: %v", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+b.cfg.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		log.Printf("bot: post message: %v", err)
		return
	}
	if err := resp.Body.Close(); err != nil {
		log.Printf("bot: close post response body: %v", err)
	}
}

// resolveIdentity fetches the bot's own user ID from the REST API.
func (b *Bot) resolveIdentity(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		b.cfg.URL+"/api/v4/users/me", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+b.cfg.Token)

	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			log.Printf("bot: close identity response body: %v", cerr)
		}
	}()

	var user struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return fmt.Errorf("decoding user response: %w", err)
	}
	if user.ID == "" {
		return fmt.Errorf("empty user ID (check token)")
	}
	b.userID = user.ID
	return nil
}

// resolveChannel finds the channel ID for the configured channel name.
func (b *Bot) resolveChannel(ctx context.Context) error {
	// Channel name may be "team/channel" or just "channel-name-with-id".
	// Try direct lookup by name: GET /api/v4/channels/name/{team}/{channel}
	// For simplicity, accept either "teamname/channelname" or a raw channel ID.
	if strings.Contains(b.cfg.Channel, "/") {
		parts := strings.SplitN(b.cfg.Channel, "/", 2)
		return b.resolveChannelByName(ctx, parts[0], parts[1])
	}
	// Treat as a raw channel ID.
	b.channelID = b.cfg.Channel
	return nil
}

func (b *Bot) resolveChannelByName(ctx context.Context, team, channel string) error {
	url := fmt.Sprintf("%s/api/v4/teams/name/%s/channels/name/%s",
		b.cfg.URL, team, channel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+b.cfg.Token)

	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			log.Printf("bot: close channel response body: %v", cerr)
		}
	}()

	var ch struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ch); err != nil {
		return fmt.Errorf("decoding channel response: %w", err)
	}
	if ch.ID == "" {
		return fmt.Errorf("channel %q not found in team %q", channel, team)
	}
	b.channelID = ch.ID
	return nil
}

// wsEvent is the minimal shape of a Mattermost WebSocket event payload.
type wsEvent struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

type wsPostData struct {
	Post      string `json:"post"` // JSON-encoded post object
	ChannelID string `json:"channel_id"`
}

type wsPost struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	ChannelID string `json:"channel_id"`
	Message   string `json:"message"`
}

// listenWebSocket opens a WebSocket connection and processes incoming events.
func (b *Bot) listenWebSocket(ctx context.Context) error {
	wsURL := strings.Replace(b.cfg.URL, "https://", "wss://", 1)
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1)
	wsURL += "/api/v4/websocket"

	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	header := http.Header{"Authorization": {"Bearer " + b.cfg.Token}}

	conn, _, err := dialer.DialContext(ctx, wsURL, header)
	if err != nil {
		return fmt.Errorf("websocket dial: %w", err)
	}
	defer func() {
		if cerr := conn.Close(); cerr != nil {
			log.Printf("bot: close websocket: %v", cerr)
		}
	}()

	log.Printf("bot: connected to %s, watching channel %s", b.cfg.URL, b.channelID)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				if ctx.Err() == nil {
					log.Printf("bot: websocket read: %v", err)
				}
				return
			}
			b.handleWSMessage(ctx, msg)
		}
	}()

	select {
	case <-ctx.Done():
		if err := conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
			log.Printf("bot: send close frame: %v", err)
		}
		return nil
	case <-done:
		return fmt.Errorf("websocket closed unexpectedly")
	}
}

func (b *Bot) handleWSMessage(ctx context.Context, raw []byte) {
	var ev wsEvent
	if err := json.Unmarshal(raw, &ev); err != nil {
		return
	}
	if ev.Event != "posted" {
		return
	}

	var data wsPostData
	if err := json.Unmarshal(ev.Data, &data); err != nil {
		return
	}
	if data.ChannelID != b.channelID {
		return
	}

	var post wsPost
	if err := json.Unmarshal([]byte(data.Post), &post); err != nil {
		return
	}
	if post.UserID == b.userID {
		return // ignore own messages
	}

	cmd := ParseCommand(post.Message)
	if cmd == nil {
		return
	}

	var response string
	switch strings.ToLower(cmd.Action) {
	case "status":
		response = b.cfg.Handler.HandleStatus()
	case "deploy":
		response = b.cfg.Handler.HandleDeploy(cmd.Service)
	case "restart":
		response = b.cfg.Handler.HandleRestart(cmd.Service)
	case "confirm":
		response = b.cfg.Handler.HandleConfirm(cmd.Service)
	default:
		response = fmt.Sprintf("unknown command: %q", cmd.Action)
	}

	b.PostMessage(ctx, response)
}
