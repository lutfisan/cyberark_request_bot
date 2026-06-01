package whitelist

import (
	"sync"
)

type Whitelist struct {
	allowedUsers  map[int64]struct{}
	allowedGroups map[int64]struct{}
	mu            sync.RWMutex
}

func NewWhitelist(allowedUsers, allowedGroups []int64) *Whitelist {
	w := &Whitelist{
		allowedUsers:  make(map[int64]struct{}),
		allowedGroups: make(map[int64]struct{}),
	}
	w.Load(allowedUsers, allowedGroups)
	return w
}

func (w *Whitelist) Load(allowedUsers, allowedGroups []int64) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.allowedUsers = make(map[int64]struct{})
	for _, id := range allowedUsers {
		w.allowedUsers[id] = struct{}{}
	}

	w.allowedGroups = make(map[int64]struct{})
	for _, id := range allowedGroups {
		w.allowedGroups[id] = struct{}{}
	}
}

func (w *Whitelist) IsAllowed(senderID int64) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	_, okUser := w.allowedUsers[senderID]
	_, okGroup := w.allowedGroups[senderID]
	return okUser || okGroup
}
