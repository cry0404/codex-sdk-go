package codex

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestRunReturnsCompletedTurnAndStoresThreadID(t *testing.T) {
	runner := &fakeRunner{
		lines: []string{
			`{"type":"thread.started","thread_id":"thread_123"}`,
			`{"type":"turn.started"}`,
			`{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"Hi!"}}`,
			`{"type":"turn.completed","usage":{"input_tokens":42,"cached_input_tokens":12,"output_tokens":5,"reasoning_output_tokens":0}}`,
		},
	}
	client := newCodexWithRunner(CodexOptions{}, runner)

	thread := client.StartThread()
	turn, err := thread.Run(context.Background(), TextInput("Hello, world!"), TurnOptions{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if thread.ID() != "thread_123" {
		t.Fatalf("thread.ID = %q, want thread_123", thread.ID())
	}
	if turn.FinalResponse != "Hi!" {
		t.Fatalf("FinalResponse = %q, want Hi!", turn.FinalResponse)
	}
	if len(turn.Items) != 1 {
		t.Fatalf("len(Items) = %d, want 1", len(turn.Items))
	}
	if turn.Usage == nil || turn.Usage.InputTokens != 42 {
		t.Fatalf("Usage = %#v", turn.Usage)
	}
	if runner.calls[0].Input != "Hello, world!" {
		t.Fatalf("Exec input = %q, want Hello, world!", runner.calls[0].Input)
	}
}

func TestRunUsesThreadIDWhenCalledTwice(t *testing.T) {
	runner := &fakeRunner{
		lines: []string{
			`{"type":"thread.started","thread_id":"thread_123"}`,
			`{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":0,"reasoning_output_tokens":0}}`,
		},
	}
	client := newCodexWithRunner(CodexOptions{}, runner)
	thread := client.StartThread()

	if _, err := thread.Run(context.Background(), TextInput("first"), TurnOptions{}); err != nil {
		t.Fatalf("first Run returned error: %v", err)
	}
	if _, err := thread.Run(context.Background(), TextInput("second"), TurnOptions{}); err != nil {
		t.Fatalf("second Run returned error: %v", err)
	}

	if len(runner.calls) != 2 {
		t.Fatalf("runner calls = %d, want 2", len(runner.calls))
	}
	if runner.calls[0].ThreadID != "" {
		t.Fatalf("first ThreadID = %q, want empty", runner.calls[0].ThreadID)
	}
	if runner.calls[1].ThreadID != "thread_123" {
		t.Fatalf("second ThreadID = %q, want thread_123", runner.calls[1].ThreadID)
	}
}

func TestRunStreamedStartsLazilyWhenEventsAreConsumed(t *testing.T) {
	runner := &fakeRunner{
		lines: []string{
			`{"type":"thread.started","thread_id":"thread_123"}`,
			`{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":0,"reasoning_output_tokens":0}}`,
		},
	}
	client := newCodexWithRunner(CodexOptions{}, runner)
	thread := client.StartThread()

	stream, err := thread.RunStreamed(context.Background(), TextInput("Hello"), TurnOptions{})
	if err != nil {
		t.Fatalf("RunStreamed returned error: %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("runner calls before consumption = %d, want 0", len(runner.calls))
	}

	_, ok, err := stream.Next(context.Background())
	if err != nil {
		t.Fatalf("Next returned error: %v", err)
	}
	if !ok {
		t.Fatal("Next ok = false, want true")
	}
	if len(runner.calls) != 1 {
		t.Fatalf("runner calls after consumption = %d, want 1", len(runner.calls))
	}
}

func TestRunNormalizesStructuredInputAndImages(t *testing.T) {
	runner := &fakeRunner{
		lines: []string{
			`{"type":"thread.started","thread_id":"thread_123"}`,
			`{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":0,"reasoning_output_tokens":0}}`,
		},
	}
	client := newCodexWithRunner(CodexOptions{}, runner)

	thread := client.StartThread()
	_, err := thread.Run(context.Background(), StructuredInput(
		Text("Describe file changes"),
		Text("Focus on impacted tests"),
		LocalImage("/tmp/first.png"),
		LocalImage("/tmp/second.jpg"),
	), TurnOptions{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	call := runner.calls[0]
	if call.Input != "Describe file changes\n\nFocus on impacted tests" {
		t.Fatalf("Input = %q", call.Input)
	}
	wantImages := []string{"/tmp/first.png", "/tmp/second.jpg"}
	if strings.Join(call.Images, ",") != strings.Join(wantImages, ",") {
		t.Fatalf("Images = %#v, want %#v", call.Images, wantImages)
	}
}

func TestRunForwardsThreadOptionsAndTurnSchema(t *testing.T) {
	runner := &fakeRunner{
		lines: []string{
			`{"type":"thread.started","thread_id":"thread_123"}`,
			`{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":0,"reasoning_output_tokens":0}}`,
		},
	}
	client := newCodexWithRunner(CodexOptions{
		BaseURL: "https://example.test",
		APIKey:  "test-key",
	}, runner)

	network := true
	thread := client.StartThread(ThreadOptions{
		Model:                 "gpt-test-1",
		SandboxMode:           SandboxWorkspaceWrite,
		WorkingDirectory:      "/tmp/project",
		SkipGitRepoCheck:      true,
		ModelReasoningEffort:  ReasoningHigh,
		NetworkAccessEnabled:  &network,
		WebSearchMode:         WebSearchLive,
		ApprovalPolicy:        ApprovalOnRequest,
		AdditionalDirectories: []string{"../backend"},
	})
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"answer": map[string]any{"type": "string"},
		},
	}
	_, err := thread.Run(context.Background(), TextInput("structured"), TurnOptions{OutputSchema: schema})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	call := runner.calls[0]
	if call.BaseURL != "https://example.test" || call.APIKey != "test-key" {
		t.Fatalf("BaseURL/APIKey not forwarded: %#v", call)
	}
	if call.OutputSchemaFile == "" {
		t.Fatal("OutputSchemaFile is empty")
	}
	data, err := os.ReadFile(call.OutputSchemaFile)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("schema file should be removed after Run, read err = %v, data = %s", err, data)
	}
	if runner.schemaDuringRun == "" {
		t.Fatal("fake runner did not observe schema file during run")
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(runner.schemaDuringRun), &decoded); err != nil {
		t.Fatalf("schema during run is not JSON: %v", err)
	}
	if decoded["type"] != "object" {
		t.Fatalf("decoded schema type = %v, want object", decoded["type"])
	}
}

func TestRunAcceptsTypedMapOutputSchema(t *testing.T) {
	runner := &fakeRunner{
		lines: []string{
			`{"type":"thread.started","thread_id":"thread_123"}`,
			`{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":0,"reasoning_output_tokens":0}}`,
		},
	}
	client := newCodexWithRunner(CodexOptions{}, runner)
	thread := client.StartThread()

	_, err := thread.Run(context.Background(), TextInput("structured"), TurnOptions{
		OutputSchema: map[string]string{"type": "object"},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if runner.schemaDuringRun == "" {
		t.Fatal("fake runner did not observe schema file during run")
	}
}

func TestRunStreamedYieldsEventsAndReportsErrors(t *testing.T) {
	runner := &fakeRunner{
		lines: []string{
			`{"type":"thread.started","thread_id":"thread_123"}`,
			`{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"Hi!"}}`,
			`not json`,
		},
	}
	client := newCodexWithRunner(CodexOptions{}, runner)

	thread := client.StartThread()
	stream, err := thread.RunStreamed(context.Background(), TextInput("Hello"), TurnOptions{})
	if err != nil {
		t.Fatalf("RunStreamed returned error: %v", err)
	}

	var events []ThreadEvent
	for event := range stream.Events() {
		events = append(events, event)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if thread.ID() != "thread_123" {
		t.Fatalf("thread.ID = %q, want thread_123", thread.ID())
	}
	if err := stream.Err(); err == nil || !strings.Contains(err.Error(), "failed to parse event") {
		t.Fatalf("stream.Err() = %v, want parse error", err)
	}
}

func TestRunStreamedNextReturnsStreamError(t *testing.T) {
	runner := &fakeRunner{
		lines: []string{
			`{"type":"thread.started","thread_id":"thread_123"}`,
			`not json`,
		},
	}
	client := newCodexWithRunner(CodexOptions{}, runner)

	thread := client.StartThread()
	stream, err := thread.RunStreamed(context.Background(), TextInput("Hello"), TurnOptions{})
	if err != nil {
		t.Fatalf("RunStreamed returned error: %v", err)
	}
	event, ok, err := stream.Next(context.Background())
	if err != nil {
		t.Fatalf("first Next returned error: %v", err)
	}
	if !ok {
		t.Fatal("first Next ok = false, want true")
	}
	if event.Type != EventThreadStarted {
		t.Fatalf("event.Type = %q, want thread.started", event.Type)
	}
	_, ok, err = stream.Next(context.Background())
	if err == nil || !strings.Contains(err.Error(), "failed to parse event") {
		t.Fatalf("second Next error = %v, want parse error", err)
	}
	if ok {
		t.Fatal("second Next ok = true, want false")
	}
}

func TestRunReturnsContextErrorWhenAlreadyCanceled(t *testing.T) {
	runner := &fakeRunner{
		lines: []string{`{"type":"thread.started","thread_id":"thread_123"}`},
	}
	client := newCodexWithRunner(CodexOptions{}, runner)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	thread := client.StartThread()
	_, err := thread.Run(ctx, TextInput("Hello"), TurnOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run error = %v, want context.Canceled", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("runner calls = %d, want 0", len(runner.calls))
	}
}

func TestRunStreamedNextReturnsContextErrorWhenAlreadyCanceled(t *testing.T) {
	runner := &fakeRunner{
		lines: []string{`{"type":"thread.started","thread_id":"thread_123"}`},
	}
	client := newCodexWithRunner(CodexOptions{}, runner)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	thread := client.StartThread()
	stream, err := thread.RunStreamed(ctx, TextInput("Hello"), TurnOptions{})
	if err != nil {
		t.Fatalf("RunStreamed returned error: %v", err)
	}
	_, ok, err := stream.Next(context.Background())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Next error = %v, want context.Canceled", err)
	}
	if ok {
		t.Fatal("Next ok = true, want false")
	}
}

func TestRunReturnsTurnFailure(t *testing.T) {
	runner := &fakeRunner{
		lines: []string{
			`{"type":"thread.started","thread_id":"thread_123"}`,
			`{"type":"turn.failed","error":{"message":"rate limit exceeded"}}`,
		},
	}
	client := newCodexWithRunner(CodexOptions{}, runner)

	thread := client.StartThread()
	_, err := thread.Run(context.Background(), TextInput("fail"), TurnOptions{})
	if err == nil || !strings.Contains(err.Error(), "rate limit exceeded") {
		t.Fatalf("Run error = %v, want rate limit exceeded", err)
	}
}

func TestRunIgnoresTopLevelErrorEventLikeTypeScriptSDK(t *testing.T) {
	runner := &fakeRunner{
		lines: []string{
			`{"type":"thread.started","thread_id":"thread_123"}`,
			`{"type":"error","message":"transient stream warning"}`,
			`{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"still completed"}}`,
			`{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":1,"reasoning_output_tokens":0}}`,
		},
	}
	client := newCodexWithRunner(CodexOptions{}, runner)

	thread := client.StartThread()
	turn, err := thread.Run(context.Background(), TextInput("warn"), TurnOptions{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if turn.FinalResponse != "still completed" {
		t.Fatalf("FinalResponse = %q, want still completed", turn.FinalResponse)
	}
}

type fakeRunner struct {
	lines           []string
	err             error
	calls           []ExecArgs
	schemaDuringRun string
}

func (f *fakeRunner) Run(ctx context.Context, args ExecArgs, emit func(string) error) error {
	f.calls = append(f.calls, args)
	if args.OutputSchemaFile != "" {
		data, err := os.ReadFile(args.OutputSchemaFile)
		if err == nil {
			f.schemaDuringRun = string(data)
		}
	}
	for _, line := range f.lines {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := emit(line); err != nil {
			return err
		}
	}
	return f.err
}
