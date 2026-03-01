package sessionmgr

import (
	"log"
	"sync"
	"time"

	"github.com/swoopsh/swoops/pkg/tmux"
	"github.com/swoopsh/swoops/server/internal/store"
)

// OutputStreamer polls a tmux pane for output and notifies subscribers.
type OutputStreamer struct {
	sessionID       string
	tmuxSessionName string
	tmuxRunner      *tmux.Runner
	store           *store.Store

	mu          sync.RWMutex
	lastOutput  string
	subscribers map[chan string]struct{}
	done        chan struct{}
	stopped     bool
}

// NewOutputStreamer creates a new output streamer for a session.
func NewOutputStreamer(sessionID, tmuxSessionName string, runner *tmux.Runner, s *store.Store) *OutputStreamer {
	return &OutputStreamer{
		sessionID:       sessionID,
		tmuxSessionName: tmuxSessionName,
		tmuxRunner:      runner,
		store:           s,
		subscribers:     make(map[chan string]struct{}),
		done:            make(chan struct{}),
	}
}

// Start begins polling the tmux pane for output changes.
func (o *OutputStreamer) Start() {
	go o.pollLoop()
}

// Stop halts the output streamer.
func (o *OutputStreamer) Stop() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.stopped {
		o.stopped = true
		close(o.done)
		// Close all subscriber channels
		for ch := range o.subscribers {
			close(ch)
			delete(o.subscribers, ch)
		}
	}
}

// Subscribe returns a channel that receives output updates.
// The caller must call Unsubscribe when done.
func (o *OutputStreamer) Subscribe() chan string {
	o.mu.Lock()
	defer o.mu.Unlock()
	ch := make(chan string, 16)
	o.subscribers[ch] = struct{}{}
	// Send current output immediately
	if o.lastOutput != "" {
		select {
		case ch <- o.lastOutput:
		default:
		}
	}
	return ch
}

// Unsubscribe removes a subscriber channel.
func (o *OutputStreamer) Unsubscribe(ch chan string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if _, ok := o.subscribers[ch]; ok {
		delete(o.subscribers, ch)
		// Don't close the channel here — the subscriber may still be draining
	}
}

func (o *OutputStreamer) pollLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-o.done:
			return
		case <-ticker.C:
			o.captureAndBroadcast()
		}
	}
}

func (o *OutputStreamer) captureAndBroadcast() {
	output, err := o.tmuxRunner.CapturePane(o.tmuxSessionName, 500)
	if err != nil {
		log.Printf("output streamer: capture pane for %s: %v", o.sessionID, err)
		return
	}

	o.mu.Lock()
	changed := output != o.lastOutput
	o.lastOutput = output
	o.mu.Unlock()

	if !changed {
		return
	}

	// Persist to DB (don't fail on error, just log)
	if err := o.store.UpdateSessionOutput(o.sessionID, output); err != nil {
		log.Printf("output streamer: update output for %s: %v", o.sessionID, err)
	}

	// Broadcast to subscribers
	o.mu.RLock()
	defer o.mu.RUnlock()
	for ch := range o.subscribers {
		select {
		case ch <- output:
		default:
			// Subscriber is slow, skip this update
		}
	}
}
