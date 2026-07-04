package codex

const (
	EventThreadStarted = "thread.started"
	EventTurnStarted   = "turn.started"
	EventTurnCompleted = "turn.completed"
	EventTurnFailed    = "turn.failed"
	EventItemStarted   = "item.started"
	EventItemUpdated   = "item.updated"
	EventItemCompleted = "item.completed"
	EventError         = "error"
)

type Usage struct {
	InputTokens           int `json:"input_tokens"`
	CachedInputTokens     int `json:"cached_input_tokens"`
	OutputTokens          int `json:"output_tokens"`
	ReasoningOutputTokens int `json:"reasoning_output_tokens"`
}

type ThreadError struct {
	Message string `json:"message"`
}

type ThreadEvent struct {
	Type     string       `json:"type"`
	ThreadID string       `json:"thread_id,omitempty"`
	Usage    *Usage       `json:"usage,omitempty"`
	Error    *ThreadError `json:"error,omitempty"`
	Message  string       `json:"message,omitempty"`
	Item     *ThreadItem  `json:"item,omitempty"`
}
