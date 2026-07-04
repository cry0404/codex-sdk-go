package codex

type ConfigObject map[string]any

type CodexOptions struct {
	CodexPathOverride string
	BaseURL           string
	APIKey            string
	Config            ConfigObject
	Env               map[string]string
}

type ApprovalMode string

const (
	ApprovalNever     ApprovalMode = "never"
	ApprovalOnRequest ApprovalMode = "on-request"
	ApprovalOnFailure ApprovalMode = "on-failure"
	ApprovalUntrusted ApprovalMode = "untrusted"
)

type SandboxMode string

const (
	SandboxReadOnly         SandboxMode = "read-only"
	SandboxWorkspaceWrite   SandboxMode = "workspace-write"
	SandboxDangerFullAccess SandboxMode = "danger-full-access"
)

type ModelReasoningEffort string

const (
	ReasoningMinimal ModelReasoningEffort = "minimal"
	ReasoningLow     ModelReasoningEffort = "low"
	ReasoningMedium  ModelReasoningEffort = "medium"
	ReasoningHigh    ModelReasoningEffort = "high"
	ReasoningXHigh   ModelReasoningEffort = "xhigh"
)

type WebSearchMode string

const (
	WebSearchDisabled WebSearchMode = "disabled"
	WebSearchCached   WebSearchMode = "cached"
	WebSearchLive     WebSearchMode = "live"
)

type ThreadOptions struct {
	Model                 string
	SandboxMode           SandboxMode
	WorkingDirectory      string
	SkipGitRepoCheck      bool
	ModelReasoningEffort  ModelReasoningEffort
	NetworkAccessEnabled  *bool
	WebSearchMode         WebSearchMode
	WebSearchEnabled      *bool
	ApprovalPolicy        ApprovalMode
	AdditionalDirectories []string
}

type TurnOptions struct {
	OutputSchema any
}
