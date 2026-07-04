# codex-sdk-go

Go SDK for embedding the Codex agent by wrapping the local `codex` CLI.

This module tracks the official TypeScript SDK at
`https://github.com/openai/codex/tree/main/sdk/typescript`. The current local
baseline is commit `98d28aab54ed86714901b6619400598598876dd0` of
`openai/codex`, scoped only to `sdk/typescript`.

The first version matches the TypeScript SDK protocol: resolve the packaged
Codex CLI from the `@openai/codex` platform package layout, spawn
`codex exec --experimental-json`, write the prompt to stdin, and parse
newline-delimited JSON events from stdout.

It intentionally does not call the OpenAI HTTP API directly and does not include
WeStock-specific agent orchestration.

## Quickstart

```go
client, err := codex.New(codex.CodexOptions{})
if err != nil {
    return err
}

thread := client.StartThread(codex.ThreadOptions{
    SandboxMode: codex.SandboxWorkspaceWrite,
})

turn, err := thread.Run(ctx, codex.TextInput("Summarize repository status"), codex.TurnOptions{})
if err != nil {
    return err
}

fmt.Println(turn.FinalResponse)
```

Use `CodexOptions.CodexPathOverride` when the Codex CLI is managed outside the
official npm package layout.

## Real CLI smoke test

The normal test suite does not require a Codex login, network access, or a local
`@openai/codex` npm install. To verify the SDK against a real logged-in Codex
CLI, run the opt-in integration test:

```sh
CODEX_SDK_GO_REAL=1 CODEX_PATH="$(command -v codex)" go test -run TestRealCodexRunStreamed -count=1
```

If `CODEX_PATH` is omitted, the test uses `codex` from `PATH`. If the CLI is not
logged in, run `codex login` first or provide `CODEX_API_KEY` in the environment.
