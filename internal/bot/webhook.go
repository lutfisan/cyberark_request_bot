package bot

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type WebhookServer struct {
	server *http.Server
	bot    *tgbotapi.BotAPI
}

func StartWebhookServer(
	bot *tgbotapi.BotAPI, 
	addr string, 
	secretToken string, 
	certPath string, 
	keyPath string, 
	dispatcher *Dispatcher,
) (*WebhookServer, error) {

	mux := http.NewServeMux()
	
	// Create a handler that validates the secret token and passes to tgbotapi
	handler := func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Telegram-Bot-Api-Secret-Token")
		if subtle.ConstantTimeCompare([]byte(token), []byte(secretToken)) != 1 {
			slog.Warn("webhook unauthenticated request", "ip", r.RemoteAddr)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		update, err := bot.HandleUpdate(r)
		if err != nil {
			slog.Error("failed to parse webhook update", "error", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		go dispatcher.ProcessUpdate(*update)
	}

	// Register webhook endpoint. For security, often a random path is used, 
	// but secret token is the primary defense here.
	mux.HandleFunc("/", handler)

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	ws := &WebhookServer{
		server: srv,
		bot:    bot,
	}

	go func() {
		slog.Info("starting webhook server", "addr", addr)
		var err error
		if certPath != "" && keyPath != "" {
			err = srv.ListenAndServeTLS(certPath, keyPath)
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			slog.Error("webhook server failed", "error", err)
		}
	}()

	return ws, nil
}

func (ws *WebhookServer) Shutdown(ctx context.Context) error {
	slog.Info("shutting down webhook server")
	
	// Call deleteWebhook to stop Telegram from pushing
	_, err := ws.bot.Request(tgbotapi.DeleteWebhookConfig{})
	if err != nil {
		slog.Warn("failed to delete webhook", "error", err)
	}

	// Shutdown HTTP server gracefully
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	
	return ws.server.Shutdown(shutdownCtx)
}
