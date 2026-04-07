package app

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/shishberg/matterops/internal/bot"
	"github.com/shishberg/matterops/internal/config"
	"github.com/shishberg/matterops/internal/dashboard"
	"github.com/shishberg/matterops/internal/service"
	"github.com/shishberg/matterops/internal/webhook"
)

// botNotifier implements service.Notifier by posting messages via the bot.
type botNotifier struct {
	bot *bot.Bot
}

func (n *botNotifier) DeployStarted(svc string) error {
	n.bot.PostMessage(context.Background(), fmt.Sprintf("Deploying `%s`...", svc))
	return nil
}

func (n *botNotifier) DeploySucceeded(svc string, output string) error {
	n.bot.PostMessage(context.Background(), fmt.Sprintf("Deploy of `%s` succeeded.", svc))
	return nil
}

func (n *botNotifier) DeployFailed(svc string, step string, output string) error {
	msg := fmt.Sprintf("Deploy of `%s` failed at step `%s`.\n```\n%s\n```", svc, step, output)
	n.bot.PostMessage(context.Background(), msg)
	return nil
}

func (n *botNotifier) DeployQueued(svc string) error {
	n.bot.PostMessage(context.Background(), fmt.Sprintf("Deploy queued for `%s`.", svc))
	return nil
}

// App wires all components together.
type App struct {
	cfg           *config.Config
	env           *config.Env
	manager       *service.Manager
	confirmations *service.ConfirmationTracker
	bot           *bot.Bot
	webhookSrv    *http.Server
	dashboardSrv  *http.Server
	cancel        context.CancelFunc
}

// New loads config and env, creates all components, and returns a ready App.
// templatesFS must contain index.html at its root.
func New(configPath string, envPath string, templatesFS fs.FS) (*App, error) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	env, err := config.LoadEnv(envPath)
	if err != nil {
		return nil, fmt.Errorf("loading env: %w", err)
	}

	svcs, err := config.LoadServices(cfg.ServicesDir)
	if err != nil {
		return nil, fmt.Errorf("loading services: %w", err)
	}

	a := &App{
		cfg:           cfg,
		env:           env,
		confirmations: service.NewConfirmationTracker(10 * time.Minute),
	}

	a.manager, err = service.NewManager(svcs, nil)
	if err != nil {
		return nil, fmt.Errorf("creating manager: %w", err)
	}

	whHandler := webhook.NewHandler(env.WebhookSecret, a)
	webhookMux := http.NewServeMux()
	webhookMux.Handle("POST /webhook/github", whHandler)

	a.webhookSrv = &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Webhook.Port),
		Handler:      webhookMux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	dash, err := dashboard.New(a.manager, templatesFS)
	if err != nil {
		return nil, fmt.Errorf("creating dashboard: %w", err)
	}

	a.dashboardSrv = &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Dashboard.Port),
		Handler:      dash,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	a.bot = bot.New(bot.Config{
		URL:     cfg.Mattermost.URL,
		Token:   env.MattermostToken,
		Channel: cfg.Mattermost.Channel,
		Handler: a,
	})

	// Wire up the notifier now that both manager and bot exist.
	a.manager.SetNotifier(&botNotifier{bot: a.bot})

	return a, nil
}

// Run starts HTTP servers and the bot, blocking until ctx is cancelled.
func (a *App) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	a.cancel = cancel

	go func() {
		log.Printf("webhook server listening on %s", a.webhookSrv.Addr)
		if err := a.webhookSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("webhook server error: %v", err)
		}
	}()

	go func() {
		log.Printf("dashboard server listening on %s", a.dashboardSrv.Addr)
		if err := a.dashboardSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("dashboard server error: %v", err)
		}
	}()

	go func() {
		if err := a.bot.Run(ctx); err != nil {
			log.Printf("bot error: %v", err)
		}
	}()

	<-ctx.Done()
	return nil
}

// Shutdown gracefully stops all components.
func (a *App) Shutdown() {
	if a.cancel != nil {
		a.cancel()
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := a.webhookSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("webhook server shutdown: %v", err)
	}
	if err := a.dashboardSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("dashboard server shutdown: %v", err)
	}

	a.manager.Stop()
}

// HandlePush implements webhook.DeployTrigger. It finds the service matching the
// repo+branch, checks require_confirmation, and either queues a deploy or
// registers a pending confirmation and notifies via bot.
func (a *App) HandlePush(repo string, branch string) {
	svc, ok := a.manager.FindServiceByRepo(repo, branch)
	if !ok {
		log.Printf("app: no service found for repo=%s branch=%s", repo, branch)
		return
	}

	if svc.RequireConfirmation {
		a.confirmations.AddPending(svc.Name, branch)
		msg := fmt.Sprintf("Deploy queued for **%s** (repo: %s, branch: %s). "+
			"Reply `@matterops confirm %s` to proceed.", svc.Name, repo, branch, svc.Name)
		a.bot.PostMessage(context.Background(), msg)
		return
	}

	if err := a.manager.RequestDeploy(svc.Name); err != nil {
		log.Printf("app: request deploy for %s: %v", svc.Name, err)
	}
}

// HandleStatus implements bot.CommandHandler and returns a markdown list of all services.
func (a *App) HandleStatus() string {
	states := a.manager.GetAllStates()
	if len(states) == 0 {
		return "No services configured."
	}

	var sb strings.Builder
	sb.WriteString("**Service Status**\n")
	for name, state := range states {
		fmt.Fprintf(&sb, "- **%s**: %s", name, state.Status)
		if state.LastResult != "" {
			fmt.Fprintf(&sb, " (last: %s)", state.LastResult)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// HandleDeploy implements bot.CommandHandler and requests a deploy for the named service.
func (a *App) HandleDeploy(svc string) string {
	if err := a.manager.RequestDeploy(svc); err != nil {
		return fmt.Sprintf("Error deploying %s: %v", svc, err)
	}
	return fmt.Sprintf("Deploy started for **%s**.", svc)
}

// HandleRestart implements bot.CommandHandler and restarts the named service.
func (a *App) HandleRestart(svc string) string {
	if err := a.manager.RestartService(svc); err != nil {
		return fmt.Sprintf("Error restarting %s: %v", svc, err)
	}
	return fmt.Sprintf("Restarted **%s**.", svc)
}

// HandleConfirm implements bot.CommandHandler and confirms a pending deploy.
func (a *App) HandleConfirm(svc string) string {
	if !a.confirmations.Confirm(svc) {
		return fmt.Sprintf("No pending deploy confirmation for **%s** (or it expired).", svc)
	}
	if err := a.manager.RequestDeploy(svc); err != nil {
		return fmt.Sprintf("Error deploying %s: %v", svc, err)
	}
	return fmt.Sprintf("Confirmed and started deploy for **%s**.", svc)
}
