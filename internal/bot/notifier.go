package bot

import (
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"cybarbot/internal/cyberark"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type SentMessage struct {
	ChatID    int64
	MessageID int
}

type CacheEntry struct {
	SeenAt     time.Time
	LastStatus string
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
	bot             *tgbotapi.BotAPI
	auth            *cyberark.AuthManager
	pollInterval    int
	notifyOnRestart bool
	targets         []int64
	
	cache sync.Map // map[string]*CacheEntry
	stats WatcherStats
	mu    sync.RWMutex
	
	stopCh chan struct{}
}

func NewNotifier(bot *tgbotapi.BotAPI, auth *cyberark.AuthManager, pollInterval int, notifyOnRestart bool, targetUsers, targetGroups []int64) *Notifier {
	targets := make([]int64, 0, len(targetUsers)+len(targetGroups))
	targets = append(targets, targetUsers...)
	targets = append(targets, targetGroups...)
	
	return &Notifier{
		bot:             bot,
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
	
	// Count cache size
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
		// Pre-populate cache
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
	// Add jitter 10%
	jitter := float64(n.pollInterval) * 0.1
	if jitter < 1 {
		jitter = 1
	}
	
	for {
		// Random interval
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
			
			// Fetch details for notification
			detail, err := n.auth.GetIncomingRequestDetail(req.RequestID)
			if err != nil {
				slog.Warn("notifier: failed to get details for new request", "reqID", req.RequestID, "error", err)
				continue
			}
			
			text := formatNotification(*detail)
			keyboard := buildNotificationKeyboard(req.RequestID)
			
			// Fan-out dispatch
			for _, target := range n.targets {
				msg := tgbotapi.NewMessage(target, text)
				msg.ReplyMarkup = keyboard
				sentMsg, err := n.bot.Send(msg)
				if err != nil {
					slog.Warn("notifier: failed to send alert", "target", target, "reqID", req.RequestID, "error", err)
					continue
				}
				entry.Dispatches = append(entry.Dispatches, SentMessage{
					ChatID:    target,
					MessageID: sentMsg.MessageID,
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
			// Stale - actioned externally or expired
			staleCount++
			
			statusMsg := "⚠️ This request is no longer pending.\nIt was actioned externally or has expired."
			
			for _, dispatch := range entry.Dispatches {
				// We don't have the full original text here unless we store it,
				// but PRD says "replace the body with a status banner" or "edit to replace body"
				// We can just send the banner, or keep the original text and append.
				// For simplicity, we just send a new text or we should have cached the original text.
				// PRD FR-104: edit each message ... replace the body with a status banner
				
				edit := tgbotapi.NewEditMessageText(dispatch.ChatID, dispatch.MessageID, statusMsg)
				_, err := n.bot.Send(edit)
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

// Called by command handlers when bot actions a request
func (n *Notifier) UpdateNotificationMessage(reqID string, statusBanner string) {
	val, ok := n.cache.Load(reqID)
	if !ok {
		return // not in cache
	}
	
	entry := val.(*CacheEntry)
	for _, dispatch := range entry.Dispatches {
		// Just replace the whole message with the status banner as per PRD
		edit := tgbotapi.NewEditMessageText(dispatch.ChatID, dispatch.MessageID, statusBanner)
		_, err := n.bot.Send(edit)
		if err != nil {
			slog.Warn("notifier: failed to edit actioned message", "target", dispatch.ChatID, "reqID", reqID, "error", err)
		}
	}
	
	n.cache.Delete(reqID)
}
