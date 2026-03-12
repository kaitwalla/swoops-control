package agentmgr

import (
	"sync"

	"github.com/kaitwalla/swoops-control/pkg/agentrpc"
)

// CommandQueue manages pending commands for each host in-memory.
type CommandQueue struct {
	mu       sync.Mutex
	commands map[string][]*agentrpc.SessionCommand // hostID -> commands
}

// NewCommandQueue creates a new command queue.
func NewCommandQueue() *CommandQueue {
	return &CommandQueue{
		commands: make(map[string][]*agentrpc.SessionCommand),
	}
}

// Enqueue adds a command to the queue for a specific host.
func (q *CommandQueue) Enqueue(hostID string, cmd *agentrpc.SessionCommand) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.commands[hostID] = append(q.commands[hostID], cmd)
}

// DequeueAll retrieves and removes all pending commands for a host.
func (q *CommandQueue) DequeueAll(hostID string) []*agentrpc.SessionCommand {
	q.mu.Lock()
	defer q.mu.Unlock()

	commands := q.commands[hostID]
	q.commands[hostID] = nil // Clear the queue

	if commands == nil {
		return []*agentrpc.SessionCommand{}
	}
	return commands
}

// Peek returns all pending commands for a host without removing them.
func (q *CommandQueue) Peek(hostID string) []*agentrpc.SessionCommand {
	q.mu.Lock()
	defer q.mu.Unlock()

	commands := q.commands[hostID]
	if commands == nil {
		return []*agentrpc.SessionCommand{}
	}
	return commands
}

// Clear removes all commands for a specific host.
func (q *CommandQueue) Clear(hostID string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.commands, hostID)
}
