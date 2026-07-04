package codex

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
)

type Turn struct {
	Items         []ThreadItem
	FinalResponse string
	Usage         *Usage
}

type StreamedTurn struct {
	events    <-chan ThreadEvent
	done      <-chan error
	startOnce sync.Once
	errOnce   sync.Once
	start     func()
	err       error
}

func (s *StreamedTurn) Events() <-chan ThreadEvent {
	s.startOnce.Do(s.start)
	return s.events
}

func (s *StreamedTurn) Err() error {
	s.startOnce.Do(s.start)
	s.errOnce.Do(func() {
		s.err = <-s.done
	})
	return s.err
}

func (s *StreamedTurn) Next(ctx context.Context) (ThreadEvent, bool, error) {
	s.startOnce.Do(s.start)
	select {
	case event, ok := <-s.events:
		if ok {
			return event, true, nil
		}
		return ThreadEvent{}, false, s.Err()
	case <-ctx.Done():
		return ThreadEvent{}, false, ctx.Err()
	}
}

type Thread struct {
	runner        lineRunner
	options       CodexOptions
	threadOptions ThreadOptions
	id            string
}

func newThread(runner lineRunner, options CodexOptions, threadOptions ThreadOptions, id string) *Thread {
	return &Thread{
		runner:        runner,
		options:       options,
		threadOptions: threadOptions,
		id:            id,
	}
}

func (t *Thread) ID() string {
	return t.id
}

func (t *Thread) RunStreamed(ctx context.Context, input Input, turnOptions TurnOptions) (*StreamedTurn, error) {
	events := make(chan ThreadEvent)
	done := make(chan error, 1)
	stream := &StreamedTurn{events: events, done: done}
	stream.start = func() {
		go func() {
			defer close(events)
			err := t.runInternal(ctx, input, turnOptions, func(event ThreadEvent) error {
				select {
				case events <- event:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			})
			done <- err
		}()
	}

	return stream, nil
}

func (t *Thread) Run(ctx context.Context, input Input, turnOptions TurnOptions) (Turn, error) {
	var turn Turn
	var turnFailure error

	err := t.runInternal(ctx, input, turnOptions, func(event ThreadEvent) error {
		switch event.Type {
		case EventItemCompleted:
			if event.Item == nil {
				return nil
			}
			if event.Item.Type == ItemAgentMessage {
				turn.FinalResponse = event.Item.Text
			}
			turn.Items = append(turn.Items, *event.Item)
		case EventTurnCompleted:
			turn.Usage = event.Usage
		case EventTurnFailed:
			if event.Error != nil {
				turnFailure = errors.New(event.Error.Message)
			} else {
				turnFailure = errors.New("turn failed")
			}
			return errStopTurn
		}
		return nil
	})
	if errors.Is(err, errStopTurn) {
		err = nil
	}
	if err != nil {
		return Turn{}, err
	}
	if turnFailure != nil {
		return Turn{}, turnFailure
	}
	return turn, nil
}

func (t *Thread) runInternal(ctx context.Context, input Input, turnOptions TurnOptions, emit func(ThreadEvent) error) error {
	if emit == nil {
		return errNoEmitter
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	schemaFile, err := createOutputSchemaFile(turnOptions.OutputSchema)
	if err != nil {
		return err
	}
	defer func() {
		_ = schemaFile.cleanup()
	}()

	prompt, images := normalizeInput(input)
	args := ExecArgs{
		Input:                 prompt,
		BaseURL:               t.options.BaseURL,
		APIKey:                t.options.APIKey,
		ThreadID:              t.id,
		Images:                images,
		Model:                 t.threadOptions.Model,
		SandboxMode:           t.threadOptions.SandboxMode,
		WorkingDirectory:      t.threadOptions.WorkingDirectory,
		AdditionalDirectories: t.threadOptions.AdditionalDirectories,
		SkipGitRepoCheck:      t.threadOptions.SkipGitRepoCheck,
		OutputSchemaFile:      schemaFile.path,
		ModelReasoningEffort:  t.threadOptions.ModelReasoningEffort,
		NetworkAccessEnabled:  t.threadOptions.NetworkAccessEnabled,
		WebSearchMode:         t.threadOptions.WebSearchMode,
		WebSearchEnabled:      t.threadOptions.WebSearchEnabled,
		ApprovalPolicy:        t.threadOptions.ApprovalPolicy,
	}

	return t.runner.Run(ctx, args, func(line string) error {
		var event ThreadEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return errors.New("failed to parse event: " + line)
		}
		if event.Type == EventThreadStarted {
			t.id = event.ThreadID
		}
		return emit(event)
	})
}

func normalizeInput(input Input) (string, []string) {
	if input.prompt != "" || len(input.items) == 0 {
		return input.prompt, nil
	}

	var promptParts []string
	var images []string
	for _, item := range input.items {
		switch item.Type {
		case "text":
			promptParts = append(promptParts, item.Text)
		case "local_image":
			images = append(images, item.Path)
		}
	}
	return strings.Join(promptParts, "\n\n"), images
}

var errStopTurn = errors.New("stop turn")
