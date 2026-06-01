package bot

import (
	"context"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"cybarbot/internal/cyberark"
	"github.com/go-telegram/bot"
)

type SentMessage struct {
	ChatID    int64
	MessageID int
}

type CacheEntry struct {
	SeenAt     time.Time
	LastStatus int
	Dispatches []SentMessage
}

type WatcherStats struct {
	PollInterval    int
	LastPoll        time.Time
	LastSeen        int
	LastNew         int
	LastStaleEdited int
	CacheSize       int
}

type Notifier struct {
	b               *bot.Bot
	auth            *cyberark.AuthManager
	pollInterval    int
	notifyOnRestart bool
	targets         []int64
	
	cache sync.Map // map[string]*CacheEntry
	stats WatcherStats
	mu    sync.RWMutex
	
	stopCh chan struct{}
}

func NewNotifier(b *bot.Bot, auth *cyberark.AuthManager, pollInterval int, notifyOnRestart bool, targetUsers, targetGroups []int64) *Notifier {
	targets := make([]int64, 0, len(targetUsers)+len(targetGroups))
	targets = append(targets, targetUsers...)
	targets = append(targets, targetGroups...)
	
	return &Notifier{
		b:               b,
		auth:            auth,
		pollInterval:    pollInterval,
		notifyOnRestart: notifyOnRestart,
		targets:         targets,
		stopCh:          make(chan struct{}),
	}
}

func (n *Notifier) GetStats() WatcherStats {
	n.mu.RLock()
	defer n.mu.RUnlock()
	
	size := 0
	n.cache.Range(func(key, value interface{}) bool {
		size++
		return true
	})
	
	stats := n.stats
	stats.CacheSize = size
	return stats
}

func (n *Notifier) Start() {
	if !n.notifyOnRestart {
		reqs, err := n.auth.GetIncomingRequests()
		if err != nil {
			slog.Error("notifier: failed to pre-populate cache", "error", err)
		} else {
			for _, r := range reqs {
				n.cache.Store(r.RequestID, &CacheEntry{
					SeenAt:     time.Now(),
					LastStatus: r.Status,
				})
			}
			slog.Info("notifier: pre-populated cache", "count", len(reqs))
		}
	}

	go n.loop()
}

func (n *Notifier) Stop() {
	close(n.stopCh)
}

func (n *Notifier) loop() {
	jitter := float64(n.pollInterval) * 0.1
	if jitter < 1 {
		jitter = 1
	}
	
	for {
		offset := (rand.Float64() * jitter * 2) - jitter
		interval := time.Duration(float64(n.pollInterval)+offset) * time.Second
		
		select {
		case <-n.stopCh:
			return
		case <-time.After(interval):
			n.poll()
		}
	}
}

func (n *Notifier) poll() {
	reqs, err := n.auth.GetIncomingRequests()
	if err != nil {
		slog.Warn("notifier: failed to poll requests", "error", err)
		return
	}
	
	now := time.Now()
	newCount := 0
	staleCount := 0
	
	currentIDs := make(map[string]bool)
	ctx := context.Background()
	
	// Pass 1: New
	for _, req := range reqs {
		currentIDs[req.RequestID] = true
		
		_, exists := n.cache.Load(req.RequestID)
		if !exists {
			newCount++
			entry := &CacheEntry{
				SeenAt:     now,
				LastStatus: req.Status,
			}
			
			detail, err := n.auth.GetIncomingRequestDetail(req.RequestID)
			if err != nil {
				slog.Warn("notifier: failed to get details for new request", "reqID", req.RequestID, "error", err)
				continue
			}
			
			text := formatNotification(*detail)
			keyboard := buildNotificationKeyboard(req.RequestID)
			
			for _, target := range n.targets {
				msg, err := n.b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID:      target,
					Text:        text,
					ReplyMarkup: keyboard,
				})
				if err != nil {
					slog.Warn("notifier: failed to send alert", "target", target, "reqID", req.RequestID, "error", err)
					continue
				}
				entry.Dispatches = append(entry.Dispatches, SentMessage{
					ChatID:    target,
					MessageID: msg.ID,
				})
			}
			
			n.cache.Store(req.RequestID, entry)
		}
	}
	
	// Pass 2: Stale
	n.cache.Range(func(key, value interface{}) bool {
		reqID := key.(string)
		entry := value.(*CacheEntry)
		
		if !currentIDs[reqID] {
			staleCount++
			statusMsg := "⚠️ This request is no longer pending.\nIt was actioned externally or has expired."
			
			for _, dispatch := range entry.Dispatches {
				_, err := n.b.EditMessageText(ctx, &bot.EditMessageTextParams{
					ChatID:    dispatch.ChatID,
					MessageID: dispatch.MessageID,
					Text:      statusMsg,
				})
				if err != nil {
					slog.Warn("notifier: failed to edit stale message", "target", dispatch.ChatID, "reqID", reqID, "error", err)
				}
			}
			
			n.cache.Delete(reqID)
		}
		return true
	})
	
	n.mu.Lock()
	n.stats.LastPoll = now
	n.stats.PollInterval = n.pollInterval
	n.stats.LastSeen = len(reqs)
	n.stats.LastNew = newCount
	n.stats.LastStaleEdited = staleCount
	n.mu.Unlock()
}

func (n *Notifier) UpdateNotificationMessage(ctx context.Context, reqID string, statusBanner string) {
	val, ok := n.cache.Load(reqID)
	if !ok {
		return // not in cache
	}
	
	entry := val.(*CacheEntry)
	for _, dispatch := range entry.Dispatches {
		_, err := n.b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    dispatch.ChatID,
			MessageID: dispatch.MessageID,
			Text:      statusMsg(statusBanner), // Wait, where is statusMsg coming from? I'll just use statusBanner directly
		})
		if err != nil {
			slog.Warn("notifier: failed to edit actioned message", "target", dispatch.ChatID, "reqID", reqID, "error", err)
		}
	}
	
	n.cache.Delete(reqID)
}

func statusMsg(s string) string {
	return s
}
