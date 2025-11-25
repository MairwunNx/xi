package telegram

import (
	"sync"
	"time"
	"ximanager/sources/artificial"
)

// StatusAnimator handles animated status messages (Thinking..., Searching...)
type StatusAnimator struct {
	thinkingText  string
	searchingText string
	reply         *StreamingReply

	mu            sync.Mutex
	currentStatus artificial.StreamStatus
	dotIndex      int
	ticker        *time.Ticker
	stopChan      chan struct{}
	stopped       bool
}

func NewStatusAnimator(thinkingText, searchingText string, reply *StreamingReply) *StatusAnimator {
	return &StatusAnimator{
		thinkingText:  thinkingText,
		searchingText: searchingText,
		reply:         reply,
		stopChan:      make(chan struct{}),
	}
}

func (a *StatusAnimator) SetStatus(status artificial.StreamStatus) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.stopped {
		return
	}

	a.currentStatus = status
	a.dotIndex = 0

	// Start animation if not already running
	if a.ticker == nil {
		a.ticker = time.NewTicker(500 * time.Millisecond)
		go a.animate()
	}

	// Immediately show first frame
	a.updateDisplay()
}

func (a *StatusAnimator) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.stopped {
		return
	}
	a.stopped = true

	if a.ticker != nil {
		a.ticker.Stop()
		close(a.stopChan)
	}
}

func (a *StatusAnimator) animate() {
	for {
		select {
		case <-a.stopChan:
			return
		case <-a.ticker.C:
			a.mu.Lock()
			if !a.stopped {
				a.dotIndex = (a.dotIndex + 1) % 4
				a.updateDisplay()
			}
			a.mu.Unlock()
		}
	}
}

func (a *StatusAnimator) updateDisplay() {
	var baseText string
	switch a.currentStatus {
	case artificial.StreamStatusThinking:
		baseText = a.thinkingText
	case artificial.StreamStatusSearching:
		baseText = a.searchingText
	default:
		return
	}

	// Animate dots: "...", " ..", "  .", "..."
	dots := []string{"...", " ..", "  .", "..."}
	text := baseText + dots[a.dotIndex]

	a.reply.Update(text)
}

