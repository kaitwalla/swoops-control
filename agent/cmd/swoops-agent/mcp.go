package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/swoopsh/swoops/pkg/version"
)

func mcpServeCommand(args []string) error {
	fs := flag.NewFlagSet("mcp-serve", flag.ContinueOnError)
	sessionID := fs.String("session-id", "", "Session ID for this MCP server")
	serverAddr := fs.String("server", "http://127.0.0.1:8080", "Control plane HTTP address")
	apiKey := fs.String("api-key", "", "API key for control plane authentication")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *sessionID == "" {
		return errors.New("--session-id is required")
	}
	if *apiKey == "" {
		*apiKey = os.Getenv("SWOOPS_API_KEY")
	}
	if *apiKey == "" {
		return errors.New("--api-key or SWOOPS_API_KEY environment variable is required")
	}

	log.SetOutput(os.Stderr) // All logging goes to stderr, stdout is for JSON-RPC

	client := &mcpClient{
		sessionID:  *sessionID,
		serverAddr: *serverAddr,
		apiKey:     *apiKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	return runMCPServer(client)
}

type mcpClient struct {
	sessionID  string
	serverAddr string
	apiKey     string
	httpClient *http.Client
}

type reportStatusArgs struct {
	StatusType string                 `json:"status_type" jsonschema:"enum=working|idle|blocked|completed|error,description=The type of status update"`
	Message    string                 `json:"message" jsonschema:"description=Human-readable status message"`
	Details    map[string]interface{} `json:"details,omitempty" jsonschema:"description=Optional additional context"`
}

type requestReviewArgs struct {
	RequestType string   `json:"request_type" jsonschema:"enum=code|architecture|security|performance,description=The type of review being requested"`
	Title       string   `json:"title" jsonschema:"description=Short title for the review request"`
	Description string   `json:"description" jsonschema:"description=Detailed description of what needs review"`
	FilePaths   []string `json:"file_paths,omitempty" jsonschema:"description=Optional list of file paths to review"`
	Diff        string   `json:"diff,omitempty" jsonschema:"description=Optional git diff or code snippet"`
}

type coordinateArgs struct {
	ToSessionID string                 `json:"to_session_id" jsonschema:"description=Target session ID to send the message to"`
	MessageType string                 `json:"message_type" jsonschema:"enum=question|info|request|response,description=Type of message being sent"`
	Subject     string                 `json:"subject" jsonschema:"description=Subject line for the message"`
	Body        string                 `json:"body" jsonschema:"description=Message body content"`
	Context     map[string]interface{} `json:"context,omitempty" jsonschema:"description=Optional additional context"`
}

func runMCPServer(client *mcpClient) error {
	// Create MCP server with stdio transport
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "swoops-orchestrator",
		Version: version.Get().Version,
	}, nil)

	// Register report_status tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "report_status",
		Description: "Report current status and progress to the Swoops control plane. Use this to communicate what you're working on, blockers, or completion.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args reportStatusArgs) (*mcp.CallToolResult, any, error) {
		argsMap := map[string]interface{}{
			"status_type": args.StatusType,
			"message":     args.Message,
			"details":     args.Details,
		}
		result, err := client.reportStatus(ctx, argsMap)
		return result, nil, err
	})

	// Register get_task tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_task",
		Description: "Retrieve the next pending task from the Swoops control plane. Tasks are prioritized instructions, fixes, reviews, or refactors assigned to this session.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
		result, err := client.getTask(ctx)
		return result, nil, err
	})

	// Register request_review tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "request_review",
		Description: "Request human review of code, architecture, security, or performance. The request will be visible in the Swoops UI for approval.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args requestReviewArgs) (*mcp.CallToolResult, any, error) {
		argsMap := map[string]interface{}{
			"request_type": args.RequestType,
			"title":        args.Title,
			"description":  args.Description,
			"file_paths":   args.FilePaths,
			"diff":         args.Diff,
		}
		result, err := client.requestReview(ctx, argsMap)
		return result, nil, err
	})

	// Register coordinate_with_session tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "coordinate_with_session",
		Description: "Send a message to another AI agent session for coordination. Use this to ask questions, share information, or request collaboration.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args coordinateArgs) (*mcp.CallToolResult, any, error) {
		argsMap := map[string]interface{}{
			"to_session_id": args.ToSessionID,
			"message_type":  args.MessageType,
			"subject":       args.Subject,
			"body":          args.Body,
			"context":       args.Context,
		}
		result, err := client.coordinateWithSession(ctx, argsMap)
		return result, nil, err
	})

	// Start stdio server
	log.Printf("MCP server starting for session %s", client.sessionID)
	return server.Run(context.Background(), &mcp.StdioTransport{})
}

// Tool implementations

func (c *mcpClient) reportStatus(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	statusType, _ := args["status_type"].(string)
	message, _ := args["message"].(string)
	details, _ := args["details"].(map[string]interface{})

	payload := map[string]interface{}{
		"status_type": statusType,
		"message":     message,
		"details":     details,
	}

	resp, err := c.apiRequest(ctx, "POST", fmt.Sprintf("/api/v1/sessions/%s/status", c.sessionID), payload)
	if err != nil {
		return nil, fmt.Errorf("failed to report status: %w", err)
	}
	defer resp.Body.Close()

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Status reported successfully: %s", message),
			},
		},
	}, nil
}

func (c *mcpClient) getTask(ctx context.Context) (*mcp.CallToolResult, error) {
	resp, err := c.apiRequest(ctx, "GET", fmt.Sprintf("/api/v1/sessions/%s/tasks/next", c.sessionID), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get task: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: "No pending tasks available.",
				},
			},
		}, nil
	}

	var task map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return nil, fmt.Errorf("failed to decode task: %w", err)
	}

	taskJSON, _ := json.MarshalIndent(task, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Task retrieved:\n%s", string(taskJSON)),
			},
		},
	}, nil
}

func (c *mcpClient) requestReview(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	payload := map[string]interface{}{
		"request_type": args["request_type"],
		"title":        args["title"],
		"description":  args["description"],
	}
	if filePaths, ok := args["file_paths"]; ok {
		payload["file_paths"] = filePaths
	}
	if diff, ok := args["diff"]; ok {
		payload["diff"] = diff
	}

	resp, err := c.apiRequest(ctx, "POST", fmt.Sprintf("/api/v1/sessions/%s/reviews", c.sessionID), payload)
	if err != nil {
		return nil, fmt.Errorf("failed to request review: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Review request created successfully. ID: %s", result["id"]),
			},
		},
	}, nil
}

func (c *mcpClient) coordinateWithSession(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	payload := map[string]interface{}{
		"to_session_id": args["to_session_id"],
		"message_type":  args["message_type"],
		"subject":       args["subject"],
		"body":          args["body"],
	}
	if context, ok := args["context"]; ok {
		payload["context"] = context
	}

	resp, err := c.apiRequest(ctx, "POST", fmt.Sprintf("/api/v1/sessions/%s/messages", c.sessionID), payload)
	if err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Message sent to session %s", args["to_session_id"]),
			},
		},
	}, nil
}

func (c *mcpClient) apiRequest(ctx context.Context, method, path string, payload interface{}) (*http.Response, error) {
	url := c.serverAddr + path

	var body io.Reader
	if payload != nil {
		jsonData, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal payload: %w", err)
		}
		body = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return resp, nil
}
