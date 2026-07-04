package codex

import "encoding/json"

const (
	ItemCommandExecution = "command_execution"
	ItemFileChange       = "file_change"
	ItemMCPToolCall      = "mcp_tool_call"
	ItemAgentMessage     = "agent_message"
	ItemReasoning        = "reasoning"
	ItemWebSearch        = "web_search"
	ItemError            = "error"
	ItemTodoList         = "todo_list"
)

type CommandExecutionStatus string

const (
	CommandInProgress CommandExecutionStatus = "in_progress"
	CommandCompleted  CommandExecutionStatus = "completed"
	CommandFailed     CommandExecutionStatus = "failed"
)

type PatchChangeKind string

const (
	PatchAdd    PatchChangeKind = "add"
	PatchDelete PatchChangeKind = "delete"
	PatchUpdate PatchChangeKind = "update"
)

type PatchApplyStatus string

const (
	PatchCompleted PatchApplyStatus = "completed"
	PatchFailed    PatchApplyStatus = "failed"
)

type MCPToolCallStatus string

const (
	MCPToolInProgress MCPToolCallStatus = "in_progress"
	MCPToolCompleted  MCPToolCallStatus = "completed"
	MCPToolFailed     MCPToolCallStatus = "failed"
)

type FileUpdateChange struct {
	Path string          `json:"path"`
	Kind PatchChangeKind `json:"kind"`
}

type MCPToolResult struct {
	Content           []json.RawMessage `json:"content,omitempty"`
	Meta              json.RawMessage   `json:"_meta,omitempty"`
	StructuredContent json.RawMessage   `json:"structured_content,omitempty"`
}

type MCPToolError struct {
	Message string `json:"message"`
}

type TodoItem struct {
	Text      string `json:"text"`
	Completed bool   `json:"completed"`
}

type ThreadItem struct {
	ID   string `json:"id,omitempty"`
	Type string `json:"type"`

	Command          string                     `json:"command,omitempty"`
	AggregatedOutput string                     `json:"aggregated_output,omitempty"`
	ExitCode         *int                       `json:"exit_code,omitempty"`
	Status           string                     `json:"status,omitempty"`
	Changes          []FileUpdateChange         `json:"changes,omitempty"`
	Server           string                     `json:"server,omitempty"`
	Tool             string                     `json:"tool,omitempty"`
	Arguments        json.RawMessage            `json:"arguments,omitempty"`
	Result           *MCPToolResult             `json:"result,omitempty"`
	Error            *MCPToolError              `json:"error,omitempty"`
	Text             string                     `json:"text,omitempty"`
	Query            string                     `json:"query,omitempty"`
	Message          string                     `json:"message,omitempty"`
	Items            []TodoItem                 `json:"items,omitempty"`
	Unknown          map[string]json.RawMessage `json:"-"`
}

func (i *ThreadItem) UnmarshalJSON(data []byte) error {
	type alias ThreadItem
	var item alias
	if err := json.Unmarshal(data, &item); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for _, key := range []string{
		"id", "type", "command", "aggregated_output", "exit_code", "status",
		"changes", "server", "tool", "arguments", "result", "error", "text",
		"query", "message", "items",
	} {
		delete(raw, key)
	}
	*i = ThreadItem(item)
	if len(raw) > 0 {
		i.Unknown = raw
	}
	return nil
}

func (i ThreadItem) MarshalJSON() ([]byte, error) {
	type alias ThreadItem
	data, err := json.Marshal(alias(i))
	if err != nil {
		return nil, err
	}
	if len(i.Unknown) == 0 {
		return data, nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	for key, value := range i.Unknown {
		if _, exists := raw[key]; !exists {
			raw[key] = value
		}
	}
	return json.Marshal(raw)
}
