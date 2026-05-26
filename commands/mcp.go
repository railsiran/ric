// commands/mcp.go
package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"ric/dispatcher"
	"ric/version"
)

// mcpRequest is a JSON-RPC 2.0 request.
type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// mcpResponse is a JSON-RPC 2.0 response.
type mcpResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Result  any    `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// mcpTool represents an MCP tool definition.
type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// mcpServerCapabilities describes what this server supports.
var mcpServerCapabilities = map[string]any{
	"tools": map[string]any{},
}

var mcpServerInfo = map[string]any{
	"name":    "ric",
	"version": version.Version,
}

// mcpTools lists all tools exposed to AI assistants.
func mcpTools() []mcpTool {
	return []mcpTool{
		{
			Name:        "ric_version",
			Description: "Show ric CLI version and build information",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "ric_sync",
			Description: "Sync host project files to the container (excludes tmp/ and storage/)",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "ric_rails",
			Description: "Run a bin/rails command inside the container and sync back results",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"args": map[string]any{
						"type":        "array",
						"items":       map[string]string{"type": "string"},
						"description": "Arguments to pass to bin/rails (e.g. ['generate', 'controller', 'Pages'])",
					},
				},
				"required": []string{"args"},
			},
		},
		{
			Name:        "ric_container_start",
			Description: "Start the project's Docker container",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "ric_container_stop",
			Description: "Stop the project's Docker container",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "ric_migrate",
			Description: "Run database migrations inside the container",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "ric_deploy",
			Description: "Deploy the application to a remote server (first-time setup). Requires .ric/deploy.yml to be configured.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "ric_upgrade",
			Description: "Upgrade an existing deployment on the remote server (preserves database and credentials). Requires .ric/deploy.yml to be configured.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}
}

// MCP starts ric as a Model Context Protocol server over stdio.
func MCP(inputs []string, flagArgs []string) error {
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for scanner.Scan() {
		var req mcpRequest
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			writeError(encoder, nil, -32700, "Parse error: "+err.Error())
			continue
		}

		switch req.Method {
		case "initialize":
			writeResult(encoder, req.ID, map[string]any{
				"protocolVersion": "2024-11-05",
				"serverInfo":      mcpServerInfo,
				"capabilities":    mcpServerCapabilities,
			})

		case "tools/list":
			writeResult(encoder, req.ID, map[string]any{
				"tools": mcpTools(),
			})

		case "tools/call":
			var params struct {
				Name      string         `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}
			if err := json.Unmarshal(req.Params, &params); err != nil {
				writeError(encoder, req.ID, -32602, "Invalid params: "+err.Error())
				continue
			}
			result, err := callMCPTool(params.Name, params.Arguments)
			if err != nil {
				writeError(encoder, req.ID, -32000, err.Error())
				continue
			}
			writeResult(encoder, req.ID, map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": result},
				},
			})

		default:
			writeError(encoder, req.ID, -32601, "Method not found: "+req.Method)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "MCP read error: %v\n", err)
		return err
	}
	return nil
}

// callMCPTool dispatches to the appropriate command based on the tool name.
func callMCPTool(name string, args json.RawMessage) (string, error) {
	switch name {
	case "ric_version":
		return version.Info(), nil

	case "ric_sync":
		if err := Sync(nil, nil); err != nil {
			return "", err
		}
		return "Sync complete", nil

	case "ric_rails":
		var p struct {
			Args []string `json:"args"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return "", fmt.Errorf("invalid args: %w", err)
		}
		if err := Rails(p.Args, nil); err != nil {
			return "", err
		}
		return fmt.Sprintf("rails %v complete", p.Args), nil

	case "ric_container_start":
		if err := Container([]string{"start"}, nil); err != nil {
			return "", err
		}
		return "Container started", nil

	case "ric_container_stop":
		if err := Container([]string{"stop"}, nil); err != nil {
			return "", err
		}
		return "Container stopped", nil

	case "ric_migrate":
		if err := Migrate(nil, nil); err != nil {
			return "", err
		}
		return "Migrations complete", nil

	case "ric_deploy":
		if err := Deploy(nil, nil); err != nil {
			return "", err
		}
		return "Deploy complete", nil

	case "ric_upgrade":
		if err := Upgrade(nil, nil); err != nil {
			return "", err
		}
		return "Upgrade complete", nil

	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

func writeResult(enc *json.Encoder, id any, result any) {
	enc.Encode(mcpResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	})
}

func writeError(enc *json.Encoder, id any, code int, message string) {
	enc.Encode(mcpResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}{Code: code, Message: message},
	})
}

func init() {
	dispatcher.Register("mcp", MCP)
}
