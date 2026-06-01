package bot

import (
	"sync"
)

type FSMState int

const (
	StateIdle FSMState = iota
	StateWaitingConfirmReason
	StateWaitingRejectReason
	StateBulkConfirmSelect
	StateWaitingBulkConfirmReason
	StateBulkRejectSelect
	StateWaitingBulkRejectReason
)

type FSMContext struct {
	State      FSMState
	RequestID  string
	RequestIDs []string // For bulk operations
	MessageID  int      // Message ID of the prompt/keyboard, to edit it later
}

type FSMManager struct {
	states sync.Map // chatID (int64) -> *FSMContext
}

func NewFSMManager() *FSMManager {
	return &FSMManager{}
}

func (f *FSMManager) GetContext(chatID int64) *FSMContext {
	v, ok := f.states.Load(chatID)
	if ok {
		return v.(*FSMContext)
	}
	// Default context is Idle
	ctx := &FSMContext{State: StateIdle}
	f.states.Store(chatID, ctx)
	return ctx
}

func (f *FSMManager) SetState(chatID int64, state FSMState) *FSMContext {
	ctx := f.GetContext(chatID)
	ctx.State = state
	return ctx
}

func (f *FSMManager) Reset(chatID int64) {
	f.states.Store(chatID, &FSMContext{State: StateIdle})
}

func (f *FSMManager) UpdateContext(chatID int64, ctx *FSMContext) {
	f.states.Store(chatID, ctx)
}
