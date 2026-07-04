package codex

import (
	"context"
	"errors"
	"os"
	osExec "os/exec"
	"strings"
	"testing"
	"time"
)

func TestRealCodexRunStreamed(t *testing.T) {
	if os.Getenv("CODEX_SDK_GO_REAL") != "1" {
		t.Skip("set CODEX_SDK_GO_REAL=1 to run against a logged-in Codex CLI")
	}

	codexPath := os.Getenv("CODEX_PATH")
	if codexPath == "" {
		var err error
		codexPath, err = osExec.LookPath("codex")
		if err != nil {
			t.Fatalf("CODEX_PATH is empty and codex is not on PATH: %v", err)
		}
	}

	apiKey := os.Getenv("CODEX_API_KEY")
	client, err := New(CodexOptions{
		CodexPathOverride: codexPath,
		APIKey:            apiKey,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	thread := client.StartThread(ThreadOptions{
		SandboxMode:      SandboxReadOnly,
		SkipGitRepoCheck: true,
		ApprovalPolicy:   ApprovalNever,
	})
	stream, err := thread.RunStreamed(ctx, TextInput("Reply with exactly: codex-sdk-go-ok"), TurnOptions{})
	if err != nil {
		t.Fatalf("RunStreamed returned error: %v", err)
	}

	var seenThreadStarted bool
	var finalResponse string
	for {
		event, ok, err := stream.Next(ctx)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("real Codex run timed out; ensure the CLI is logged in and non-interactive: %v", err)
			}
			t.Fatalf("stream iteration returned error: %v", err)
		}
		if !ok {
			break
		}
		if event.Type == EventThreadStarted {
			seenThreadStarted = true
		}
		if event.Type == EventItemCompleted && event.Item != nil && event.Item.Type == ItemAgentMessage {
			finalResponse = event.Item.Text
		}
	}

	if !seenThreadStarted {
		t.Fatal("did not receive thread.started")
	}
	if thread.ID() == "" {
		t.Fatal("thread ID was not populated")
	}
	if !strings.Contains(strings.ToLower(finalResponse), "codex-sdk-go-ok") {
		t.Fatalf("final response = %q, want it to contain codex-sdk-go-ok", finalResponse)
	}
}
