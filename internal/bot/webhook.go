package bot

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-telegram/bot"
)

type WebhookServer struct {
	server *http.Server
	b      *bot.Bot
}

func StartWebhookServer(
	b *bot.Bot,
	addr string,
	certPath string,
	keyPath string,
) (*WebhookServer, error) {

	mux := http.NewServeMux()
	
	// The underlying library handles the secret token validation if configured correctly in bot.WithWebhook
	mux.HandleFunc("/", b.WebhookHandler())

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	ws := &WebhookServer{
		server: srv,
		b:      b,
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
	
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	
	return ws.server.Shutdown(shutdownCtx)
}
