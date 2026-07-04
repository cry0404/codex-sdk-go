package codex

import (
	"encoding/json"
	"testing"
)

func TestDecodeThreadEvents(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want func(t *testing.T, event ThreadEvent)
	}{
		{
			name: "thread started",
			raw:  `{"type":"thread.started","thread_id":"thread_123"}`,
			want: func(t *testing.T, event ThreadEvent) {
				if event.Type != EventThreadStarted {
					t.Fatalf("Type = %q, want %q", event.Type, EventThreadStarted)
				}
				if event.ThreadID != "thread_123" {
					t.Fatalf("ThreadID = %q, want thread_123", event.ThreadID)
				}
			},
		},
		{
			name: "agent message item",
			raw:  `{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"Hi"}}`,
			want: func(t *testing.T, event ThreadEvent) {
				if event.Type != EventItemCompleted {
					t.Fatalf("Type = %q, want %q", event.Type, EventItemCompleted)
				}
				if event.Item == nil {
					t.Fatal("Item is nil")
				}
				if event.Item.ID != "item_1" {
					t.Fatalf("Item.ID = %q, want item_1", event.Item.ID)
				}
				if event.Item.Type != ItemAgentMessage {
					t.Fatalf("Item.Type = %q, want %q", event.Item.Type, ItemAgentMessage)
				}
				if event.Item.Text != "Hi" {
					t.Fatalf("Item.Text = %q, want Hi", event.Item.Text)
				}
			},
		},
		{
			name: "turn completed usage",
			raw:  `{"type":"turn.completed","usage":{"input_tokens":42,"cached_input_tokens":12,"output_tokens":5,"reasoning_output_tokens":0}}`,
			want: func(t *testing.T, event ThreadEvent) {
				if event.Type != EventTurnCompleted {
					t.Fatalf("Type = %q, want %q", event.Type, EventTurnCompleted)
				}
				if event.Usage == nil {
					t.Fatal("Usage is nil")
				}
				if event.Usage.InputTokens != 42 {
					t.Fatalf("InputTokens = %d, want 42", event.Usage.InputTokens)
				}
				if event.Usage.CachedInputTokens != 12 {
					t.Fatalf("CachedInputTokens = %d, want 12", event.Usage.CachedInputTokens)
				}
				if event.Usage.OutputTokens != 5 {
					t.Fatalf("OutputTokens = %d, want 5", event.Usage.OutputTokens)
				}
				if event.Usage.ReasoningOutputTokens != 0 {
					t.Fatalf("ReasoningOutputTokens = %d, want 0", event.Usage.ReasoningOutputTokens)
				}
			},
		},
		{
			name: "mcp tool call keeps unknown payloads",
			raw:  `{"type":"item.completed","item":{"id":"call_1","type":"mcp_tool_call","server":"westock","tool":"snapshot","arguments":{"symbol":"AAPL"},"result":{"content":[{"type":"text","text":"ok"}],"_meta":{"trace":"abc"},"structured_content":{"price":123}},"status":"completed"}}`,
			want: func(t *testing.T, event ThreadEvent) {
				if event.Item == nil {
					t.Fatal("Item is nil")
				}
				if string(event.Item.Arguments) != `{"symbol":"AAPL"}` {
					t.Fatalf("Arguments = %s", event.Item.Arguments)
				}
				if event.Item.Result == nil {
					t.Fatal("Result is nil")
				}
				if len(event.Item.Result.Content) != 1 {
					t.Fatalf("len(Result.Content) = %d, want 1", len(event.Item.Result.Content))
				}
				if string(event.Item.Result.StructuredContent) != `{"price":123}` {
					t.Fatalf("StructuredContent = %s", event.Item.Result.StructuredContent)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var event ThreadEvent
			if err := json.Unmarshal([]byte(tt.raw), &event); err != nil {
				t.Fatalf("Unmarshal returned error: %v", err)
			}
			tt.want(t, event)
		})
	}
}

func TestThreadItemPreservesUnknownFieldsWhenMarshaled(t *testing.T) {
	raw := []byte(`{"id":"item_1","type":"agent_message","text":"Hi","future_field":{"nested":true}}`)
	var item ThreadItem
	if err := json.Unmarshal(raw, &item); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}

	encoded, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("Unmarshal encoded returned error: %v", err)
	}
	if string(got["future_field"]) != `{"nested":true}` {
		t.Fatalf("future_field = %s", got["future_field"])
	}
}
