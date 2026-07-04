package codex

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	osExec "os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	internalOriginatorEnv = "CODEX_INTERNAL_ORIGINATOR_OVERRIDE"
	goSDKOriginator       = "codex_sdk_go"
)

type ExecOptions struct {
	Path   string
	Env    map[string]string
	Config ConfigObject
}

type ExecArgs struct {
	Input string

	BaseURL               string
	APIKey                string
	ThreadID              string
	Images                []string
	Model                 string
	SandboxMode           SandboxMode
	WorkingDirectory      string
	AdditionalDirectories []string
	SkipGitRepoCheck      bool
	OutputSchemaFile      string
	ModelReasoningEffort  ModelReasoningEffort
	NetworkAccessEnabled  *bool
	WebSearchMode         WebSearchMode
	WebSearchEnabled      *bool
	ApprovalPolicy        ApprovalMode
}

type lineRunner interface {
	Run(ctx context.Context, args ExecArgs, emit func(string) error) error
}

type Exec struct {
	path     string
	pathDirs []string
	env      map[string]string
	config   ConfigObject
}

func NewExec(options ExecOptions) (*Exec, error) {
	path := options.Path
	var pathDirs []string
	if path == "" {
		resolved, err := FindCodexPath()
		if err != nil {
			return nil, err
		}
		path = resolved.ExecutablePath
		pathDirs = resolved.PathDirs
	}
	return &Exec{
		path:     path,
		pathDirs: pathDirs,
		env:      copyStringMap(options.Env),
		config:   copyConfigObject(options.Config),
	}, nil
}

func (e *Exec) Run(ctx context.Context, args ExecArgs, emit func(string) error) error {
	commandArgs, env, err := e.build(ctx, args)
	if err != nil {
		return err
	}

	cmd := osExec.CommandContext(ctx, e.path, commandArgs...)
	cmd.Env = flattenEnv(env)
	cmd.Stdin = bytes.NewBufferString(args.Input)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	var stderrBuffer bytes.Buffer
	if err := cmd.Start(); err != nil {
		return err
	}
	stderrDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(&stderrBuffer, stderr)
		close(stderrDone)
	}()

	if err := readJSONLines(stdout, emit); err != nil {
		_ = cmd.Process.Kill()
		<-stderrDone
		_ = cmd.Wait()
		return err
	}

	waitErr := cmd.Wait()
	<-stderrDone
	if waitErr != nil {
		return fmt.Errorf("codex exec exited: %w: %s", waitErr, stderrBuffer.String())
	}
	return nil
}

func (e *Exec) build(_ context.Context, args ExecArgs) ([]string, map[string]string, error) {
	commandArgs := []string{"exec", "--experimental-json"}

	overrides, err := serializeConfigOverrides(e.config)
	if err != nil {
		return nil, nil, err
	}
	for _, override := range overrides {
		commandArgs = append(commandArgs, "--config", override)
	}

	if args.BaseURL != "" {
		value, err := toTOMLLiteral(args.BaseURL, "openai_base_url")
		if err != nil {
			return nil, nil, err
		}
		commandArgs = append(commandArgs, "--config", "openai_base_url="+value)
	}
	if args.Model != "" {
		commandArgs = append(commandArgs, "--model", args.Model)
	}
	if args.SandboxMode != "" {
		commandArgs = append(commandArgs, "--sandbox", string(args.SandboxMode))
	}
	if args.WorkingDirectory != "" {
		commandArgs = append(commandArgs, "--cd", args.WorkingDirectory)
	}
	for _, dir := range args.AdditionalDirectories {
		commandArgs = append(commandArgs, "--add-dir", dir)
	}
	if args.SkipGitRepoCheck {
		commandArgs = append(commandArgs, "--skip-git-repo-check")
	}
	if args.OutputSchemaFile != "" {
		commandArgs = append(commandArgs, "--output-schema", args.OutputSchemaFile)
	}
	if args.ModelReasoningEffort != "" {
		commandArgs = append(commandArgs, "--config", fmt.Sprintf("model_reasoning_effort=%s", quoteString(string(args.ModelReasoningEffort))))
	}
	if args.NetworkAccessEnabled != nil {
		commandArgs = append(commandArgs, "--config", fmt.Sprintf("sandbox_workspace_write.network_access=%t", *args.NetworkAccessEnabled))
	}
	if args.WebSearchMode != "" {
		commandArgs = append(commandArgs, "--config", fmt.Sprintf("web_search=%s", quoteString(string(args.WebSearchMode))))
	} else if args.WebSearchEnabled != nil {
		mode := WebSearchDisabled
		if *args.WebSearchEnabled {
			mode = WebSearchLive
		}
		commandArgs = append(commandArgs, "--config", fmt.Sprintf("web_search=%s", quoteString(string(mode))))
	}
	if args.ApprovalPolicy != "" {
		commandArgs = append(commandArgs, "--config", fmt.Sprintf("approval_policy=%s", quoteString(string(args.ApprovalPolicy))))
	}
	if args.ThreadID != "" {
		commandArgs = append(commandArgs, "resume", args.ThreadID)
	}
	for _, image := range args.Images {
		commandArgs = append(commandArgs, "--image", image)
	}

	env := e.baseEnv()
	if _, ok := env[internalOriginatorEnv]; !ok {
		env[internalOriginatorEnv] = goSDKOriginator
	}
	if args.APIKey != "" {
		env["CODEX_API_KEY"] = args.APIKey
	}
	if len(e.pathDirs) > 0 {
		PrependPathDirs(env, e.pathDirs, runtime.GOOS)
	}
	return commandArgs, env, nil
}

func (e *Exec) baseEnv() map[string]string {
	if e.env != nil {
		return copyStringMap(e.env)
	}
	env := map[string]string{}
	for _, item := range os.Environ() {
		for i := 0; i < len(item); i++ {
			if item[i] == '=' {
				env[item[:i]] = item[i+1:]
				break
			}
		}
	}
	return env
}

func flattenEnv(env map[string]string) []string {
	items := make([]string, 0, len(env))
	for key, value := range env {
		items = append(items, key+"="+value)
	}
	return items
}

func copyStringMap(source map[string]string) map[string]string {
	if source == nil {
		return nil
	}
	target := make(map[string]string, len(source))
	for key, value := range source {
		target[key] = value
	}
	return target
}

func copyConfigObject(source ConfigObject) ConfigObject {
	if source == nil {
		return nil
	}
	target := make(ConfigObject, len(source))
	for key, value := range source {
		target[key] = value
	}
	return target
}

var errNoEmitter = errors.New("codex line emitter is nil")

func readJSONLines(reader io.Reader, emit func(string) error) error {
	var buffer bytes.Buffer
	chunk := make([]byte, 32*1024)
	for {
		n, readErr := reader.Read(chunk)
		if n > 0 {
			data := chunk[:n]
			for len(data) > 0 {
				index := bytes.IndexByte(data, '\n')
				if index < 0 {
					buffer.Write(data)
					break
				}
				buffer.Write(data[:index])
				line := buffer.String()
				if err := emit(line); err != nil {
					return err
				}
				buffer.Reset()
				data = data[index+1:]
			}
		}
		if readErr == nil {
			continue
		}
		if errors.Is(readErr, io.EOF) {
			if buffer.Len() > 0 {
				if err := emit(buffer.String()); err != nil {
					return err
				}
			}
			return nil
		}
		return readErr
	}
}

type CodexPathResolution struct {
	ExecutablePath string
	PathDirs       []string
}

var platformPackageByTarget = map[string]string{
	"x86_64-unknown-linux-musl":  "@openai/codex-linux-x64",
	"aarch64-unknown-linux-musl": "@openai/codex-linux-arm64",
	"x86_64-apple-darwin":        "@openai/codex-darwin-x64",
	"aarch64-apple-darwin":       "@openai/codex-darwin-arm64",
	"x86_64-pc-windows-msvc":     "@openai/codex-win32-x64",
	"aarch64-pc-windows-msvc":    "@openai/codex-win32-arm64",
}

func FindCodexPath() (CodexPathResolution, error) {
	return findCodexPathInSearchStarts(candidateSearchStarts(), runtime.GOOS, runtime.GOARCH)
}

func findCodexPathInSearchStarts(starts []string, goos string, goarch string) (CodexPathResolution, error) {
	targetTriple, err := targetTriple(goos, goarch)
	if err != nil {
		return CodexPathResolution{}, err
	}
	platformPackage := platformPackageByTarget[targetTriple]
	if platformPackage == "" {
		return CodexPathResolution{}, fmt.Errorf("unsupported target triple: %s", targetTriple)
	}
	binaryName := "codex"
	if goos == "windows" {
		binaryName = "codex.exe"
	}

	for _, root := range candidateVendorRoots(starts, platformPackage) {
		if resolved := ResolveNativePackage(root, targetTriple, binaryName); resolved != nil {
			return *resolved, nil
		}
	}
	return CodexPathResolution{}, errors.New("unable to locate Codex CLI binaries. Ensure @openai/codex is installed with optional dependencies")
}

func ResolveNativePackage(vendorRoot string, targetTriple string, codexBinaryName string) *CodexPathResolution {
	packageRoot := filepath.Join(vendorRoot, targetTriple)
	packageBinaryPath := filepath.Join(packageRoot, "bin", codexBinaryName)
	if isFile(packageBinaryPath) && isFile(filepath.Join(packageRoot, "codex-package.json")) {
		return &CodexPathResolution{
			ExecutablePath: packageBinaryPath,
			PathDirs:       existingDirs(filepath.Join(packageRoot, "codex-path")),
		}
	}

	legacyBinaryPath := filepath.Join(packageRoot, "codex", codexBinaryName)
	if isFile(legacyBinaryPath) {
		return &CodexPathResolution{
			ExecutablePath: legacyBinaryPath,
			PathDirs:       existingDirs(filepath.Join(packageRoot, "path")),
		}
	}
	return nil
}

func PrependPathDirs(env map[string]string, pathDirs []string, goos string) {
	pathKey := pathEnvKey(env, goos)
	if goos == "windows" {
		for key := range env {
			if strings.EqualFold(key, "path") && key != pathKey {
				delete(env, key)
			}
		}
	}

	existing := strings.Split(env[pathKey], string(os.PathListSeparator))
	filtered := make([]string, 0, len(existing))
	for _, entry := range existing {
		if entry == "" || containsString(pathDirs, entry) {
			continue
		}
		filtered = append(filtered, entry)
	}
	env[pathKey] = strings.Join(append(append([]string(nil), pathDirs...), filtered...), string(os.PathListSeparator))
}

func pathEnvKey(env map[string]string, goos string) string {
	if goos != "windows" {
		return "PATH"
	}
	var last string
	for key := range env {
		if strings.EqualFold(key, "path") {
			last = key
			if key == "Path" {
				return "Path"
			}
		}
	}
	if last != "" {
		return last
	}
	return "PATH"
}

func candidateVendorRoots(starts []string, platformPackage string) []string {
	var roots []string
	for _, start := range starts {
		dir := start
		for {
			nodeModules := filepath.Join(dir, "node_modules")
			codexPackage := filepath.Join(nodeModules, "@openai", "codex", "package.json")
			if isFile(codexPackage) {
				roots = append(roots, filepath.Join(nodeModules, filepath.FromSlash(platformPackage), "vendor"))
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return roots
}

func candidateSearchStarts() []string {
	var starts []string
	if cwd, err := os.Getwd(); err == nil {
		starts = append(starts, cwd)
	}
	if executable, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(executable))
	}
	return starts
}

func targetTriple(goos string, goarch string) (string, error) {
	switch goos {
	case "linux", "android":
		switch goarch {
		case "amd64":
			return "x86_64-unknown-linux-musl", nil
		case "arm64":
			return "aarch64-unknown-linux-musl", nil
		}
	case "darwin":
		switch goarch {
		case "amd64":
			return "x86_64-apple-darwin", nil
		case "arm64":
			return "aarch64-apple-darwin", nil
		}
	case "windows":
		switch goarch {
		case "amd64":
			return "x86_64-pc-windows-msvc", nil
		case "arm64":
			return "aarch64-pc-windows-msvc", nil
		}
	}
	return "", fmt.Errorf("unsupported platform: %s (%s)", goos, goarch)
}

func existingDirs(dirs ...string) []string {
	var existing []string
	for _, dir := range dirs {
		if isDirectory(dir) {
			existing = append(existing, dir)
		}
	}
	return existing
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
