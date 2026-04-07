package bot_test

import (
	"testing"

	"github.com/shishberg/matterops/internal/bot"
	"github.com/stretchr/testify/assert"
)

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    *bot.Command
	}{
		{"status command", "@matterops status", &bot.Command{Action: "status"}},
		{"deploy command", "@matterops deploy myapp", &bot.Command{Action: "deploy", Service: "myapp"}},
		{"restart command", "@matterops restart myapp", &bot.Command{Action: "restart", Service: "myapp"}},
		{"confirm command", "@matterops confirm myapp", &bot.Command{Action: "confirm", Service: "myapp"}},
		{"with extra whitespace", "  @matterops   deploy   myapp  ", &bot.Command{Action: "deploy", Service: "myapp"}},
		{"not a command", "hello world", nil},
		{"empty after mention", "@matterops", nil},
		{"unknown command", "@matterops foobar", &bot.Command{Action: "foobar"}},
		{"case insensitive mention", "@MatterOps deploy myapp", &bot.Command{Action: "deploy", Service: "myapp"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bot.ParseCommand(tt.message)
			assert.Equal(t, tt.want, got)
		})
	}
}
