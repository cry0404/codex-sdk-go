package codex

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestBuildExecArgsMatchesTypeScriptSDKOrder(t *testing.T) {
	network := true
	webSearch := false
	exec := mustNewExec(t, ExecOptions{
		Path: "codex",
		Env:  map[string]string{},
		Config: ConfigObject{
			"approval_policy": "never",
			"sandbox_workspace_write": ConfigObject{
				"network_access": true,
			},
			"retry_budget": 3,
			"tool_rules": ConfigObject{
				"allow": []string{"git status", "git diff"},
			},
		},
	})

	args, env, err := exec.build(context.Background(), ExecArgs{
		Input:                 "hi",
		BaseURL:               "https://example.test",
		APIKey:                "test-key",
		ThreadID:              "thread-id",
		Images:                []string{"img.png"},
		Model:                 "gpt-test-1",
		SandboxMode:           SandboxWorkspaceWrite,
		WorkingDirectory:      "/tmp/project",
		AdditionalDirectories: []string{"../backend", "/tmp/shared"},
		SkipGitRepoCheck:      true,
		OutputSchemaFile:      "/tmp/schema.json",
		ModelReasoningEffort:  ReasoningHigh,
		NetworkAccessEnabled:  &network,
		WebSearchEnabled:      &webSearch,
		ApprovalPolicy:        ApprovalOnRequest,
	})
	if err != nil {
		t.Fatalf("build returned error: %v", err)
	}

	wantPrefix := []string{
		"exec",
		"--experimental-json",
	}
	if !reflect.DeepEqual(args[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("args prefix = %#v, want %#v", args[:len(wantPrefix)], wantPrefix)
	}

	expectPair(t, args, "--config", `openai_base_url="https://example.test"`)
	expectPair(t, args, "--model", "gpt-test-1")
	expectPair(t, args, "--sandbox", "workspace-write")
	expectPair(t, args, "--cd", "/tmp/project")
	expectPair(t, args, "--add-dir", "../backend")
	expectPair(t, args, "--add-dir", "/tmp/shared")
	expectPair(t, args, "--output-schema", "/tmp/schema.json")
	expectPair(t, args, "--config", `model_reasoning_effort="high"`)
	expectPair(t, args, "--config", "sandbox_workspace_write.network_access=true")
	expectPair(t, args, "--config", `web_search="disabled"`)
	expectPair(t, args, "--config", `approval_policy="on-request"`)

	if !slices.Contains(args, "--skip-git-repo-check") {
		t.Fatal("args missing --skip-git-repo-check")
	}

	resumeIndex := slices.Index(args, "resume")
	imageIndex := slices.Index(args, "--image")
	if resumeIndex == -1 {
		t.Fatal("args missing resume")
	}
	if imageIndex == -1 {
		t.Fatal("args missing --image")
	}
	if resumeIndex > imageIndex {
		t.Fatalf("resume index %d must be before image index %d", resumeIndex, imageIndex)
	}
	if got := args[resumeIndex+1]; got != "thread-id" {
		t.Fatalf("resume thread id = %q, want thread-id", got)
	}
	if got := args[imageIndex+1]; got != "img.png" {
		t.Fatalf("image path = %q, want img.png", got)
	}

	globalApproval := collectConfigValues(args, "approval_policy")
	if !reflect.DeepEqual(globalApproval, []string{`approval_policy="never"`, `approval_policy="on-request"`}) {
		t.Fatalf("approval_policy configs = %#v", globalApproval)
	}
	if env["CODEX_API_KEY"] != "test-key" {
		t.Fatalf("CODEX_API_KEY = %q, want test-key", env["CODEX_API_KEY"])
	}
	if env["CODEX_INTERNAL_ORIGINATOR_OVERRIDE"] != "codex_sdk_go" {
		t.Fatalf("originator = %q, want codex_sdk_go", env["CODEX_INTERNAL_ORIGINATOR_OVERRIDE"])
	}
}

func TestBuildExecArgsPrefersWebSearchModeOverLegacyBoolean(t *testing.T) {
	enabled := true
	exec := mustNewExec(t, ExecOptions{Path: "codex"})

	args, _, err := exec.build(context.Background(), ExecArgs{
		Input:            "hi",
		WebSearchMode:    WebSearchCached,
		WebSearchEnabled: &enabled,
	})
	if err != nil {
		t.Fatalf("build returned error: %v", err)
	}

	values := collectConfigValues(args, "web_search")
	if !reflect.DeepEqual(values, []string{`web_search="cached"`}) {
		t.Fatalf("web_search configs = %#v, want cached only", values)
	}
}

func TestToTOMLLiteralRejectsUnsupportedValues(t *testing.T) {
	if _, err := toTOMLLiteral(nil, "bad"); err == nil {
		t.Fatal("toTOMLLiteral(nil) returned nil error")
	}
	if _, err := toTOMLLiteral(map[string]any{"": "bad"}, "bad"); err == nil {
		t.Fatal("toTOMLLiteral(empty key) returned nil error")
	}
}

func TestReadJSONLinesAcceptsLargeAggregatedOutputEvents(t *testing.T) {
	largeOutput := strings.Repeat("x", bufio.MaxScanTokenSize+1024)
	line := `{"type":"item.updated","item":{"id":"cmd_1","type":"command_execution","command":"make test","aggregated_output":"` + largeOutput + `","status":"in_progress"}}`
	var lines []string

	if err := readJSONLines(strings.NewReader(line+"\n"), func(got string) error {
		lines = append(lines, got)
		return nil
	}); err != nil {
		t.Fatalf("readJSONLines returned error: %v", err)
	}

	if len(lines) != 1 {
		t.Fatalf("len(lines) = %d, want 1", len(lines))
	}
	if lines[0] != line {
		t.Fatal("large line was not preserved")
	}
}

func TestResolveNativePackageFindsPackageLayoutBinaryAndPathDir(t *testing.T) {
	vendorRoot := t.TempDir()
	packageRoot := filepath.Join(vendorRoot, "x86_64-unknown-linux-musl")
	binDir := filepath.Join(packageRoot, "bin")
	pathDir := filepath.Join(packageRoot, "codex-path")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	if err := os.MkdirAll(pathDir, 0o755); err != nil {
		t.Fatalf("mkdir path: %v", err)
	}
	writeTestFile(t, filepath.Join(packageRoot, "codex-package.json"), "{}")
	writeTestFile(t, filepath.Join(binDir, "codex"), "")

	resolved := ResolveNativePackage(vendorRoot, "x86_64-unknown-linux-musl", "codex")
	if resolved == nil {
		t.Fatal("ResolveNativePackage returned nil")
	}
	if resolved.ExecutablePath != filepath.Join(binDir, "codex") {
		t.Fatalf("ExecutablePath = %q", resolved.ExecutablePath)
	}
	if !reflect.DeepEqual(resolved.PathDirs, []string{pathDir}) {
		t.Fatalf("PathDirs = %#v", resolved.PathDirs)
	}
}

func TestFindCodexPathUsesCodexPackageRelativePlatformPackage(t *testing.T) {
	projectRoot := t.TempDir()
	nodeModules := filepath.Join(projectRoot, "node_modules")
	codexPackageRoot := filepath.Join(nodeModules, "@openai", "codex")
	vendorRoot := filepath.Join(nodeModules, "@openai", "codex-darwin-arm64", "vendor")
	nativeRoot := filepath.Join(vendorRoot, "aarch64-apple-darwin")
	binDir := filepath.Join(nativeRoot, "bin")
	pathDir := filepath.Join(nativeRoot, "codex-path")
	if err := os.MkdirAll(codexPackageRoot, 0o755); err != nil {
		t.Fatalf("mkdir codex package: %v", err)
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	if err := os.MkdirAll(pathDir, 0o755); err != nil {
		t.Fatalf("mkdir path: %v", err)
	}
	writeTestFile(t, filepath.Join(codexPackageRoot, "package.json"), "{}")
	writeTestFile(t, filepath.Join(nativeRoot, "codex-package.json"), "{}")
	writeTestFile(t, filepath.Join(binDir, "codex"), "")

	resolved, err := findCodexPathInSearchStarts([]string{projectRoot}, "darwin", "arm64")
	if err != nil {
		t.Fatalf("findCodexPathInSearchStarts returned error: %v", err)
	}
	if resolved.ExecutablePath != filepath.Join(binDir, "codex") {
		t.Fatalf("ExecutablePath = %q", resolved.ExecutablePath)
	}
	if !reflect.DeepEqual(resolved.PathDirs, []string{pathDir}) {
		t.Fatalf("PathDirs = %#v", resolved.PathDirs)
	}
}

func TestResolveNativePackageFallsBackToLegacyLayout(t *testing.T) {
	vendorRoot := t.TempDir()
	packageRoot := filepath.Join(vendorRoot, "x86_64-unknown-linux-musl")
	binDir := filepath.Join(packageRoot, "codex")
	pathDir := filepath.Join(packageRoot, "path")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	if err := os.MkdirAll(pathDir, 0o755); err != nil {
		t.Fatalf("mkdir path: %v", err)
	}
	writeTestFile(t, filepath.Join(binDir, "codex"), "")

	resolved := ResolveNativePackage(vendorRoot, "x86_64-unknown-linux-musl", "codex")
	if resolved == nil {
		t.Fatal("ResolveNativePackage returned nil")
	}
	if resolved.ExecutablePath != filepath.Join(binDir, "codex") {
		t.Fatalf("ExecutablePath = %q", resolved.ExecutablePath)
	}
	if !reflect.DeepEqual(resolved.PathDirs, []string{pathDir}) {
		t.Fatalf("PathDirs = %#v", resolved.PathDirs)
	}
}

func TestPrependPathDirsWithoutDuplicatingEntries(t *testing.T) {
	pathDir := filepath.Join(t.TempDir(), "codex-path")
	env := map[string]string{"PATH": "/usr/bin" + string(os.PathListSeparator) + pathDir}

	PrependPathDirs(env, []string{pathDir}, "darwin")

	if env["PATH"] != pathDir+string(os.PathListSeparator)+"/usr/bin" {
		t.Fatalf("PATH = %q", env["PATH"])
	}
}

func TestPrependPathDirsPreservesWindowsPathKey(t *testing.T) {
	pathDir := filepath.Join(t.TempDir(), "codex-path")
	env := map[string]string{"PATH": "/usr/bin", "Path": "C\\Windows" + string(os.PathListSeparator) + pathDir}

	PrependPathDirs(env, []string{pathDir}, "windows")

	if _, ok := env["PATH"]; ok {
		t.Fatalf("PATH key should be removed on windows: %#v", env)
	}
	if env["Path"] != pathDir+string(os.PathListSeparator)+"C\\Windows" {
		t.Fatalf("Path = %q", env["Path"])
	}
}

func expectPair(t *testing.T, args []string, key string, value string) {
	t.Helper()
	for i := 0; i < len(args)-1; i++ {
		if args[i] == key && args[i+1] == value {
			return
		}
	}
	t.Fatalf("pair %s %s not found in %#v", key, value, args)
}

func mustNewExec(t *testing.T, options ExecOptions) *Exec {
	t.Helper()
	exec, err := NewExec(options)
	if err != nil {
		t.Fatalf("NewExec returned error: %v", err)
	}
	return exec
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write test file %s: %v", path, err)
	}
}

func collectConfigValues(args []string, key string) []string {
	var values []string
	for i := 0; i < len(args)-1; i++ {
		if args[i] != "--config" {
			continue
		}
		if value := args[i+1]; len(value) > len(key) && value[:len(key)+1] == key+"=" {
			values = append(values, value)
		}
	}
	return values
}
