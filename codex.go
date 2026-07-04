package codex

type Codex struct {
	options CodexOptions
	runner  lineRunner
}

func New(options CodexOptions) (*Codex, error) {
	exec, err := NewExec(ExecOptions{
		Path:   options.CodexPathOverride,
		Env:    options.Env,
		Config: options.Config,
	})
	if err != nil {
		return nil, err
	}
	return &Codex{
		options: options,
		runner:  exec,
	}, nil
}

func newCodexWithRunner(options CodexOptions, runner lineRunner) *Codex {
	return &Codex{
		options: options,
		runner:  runner,
	}
}

func (c *Codex) StartThread(options ...ThreadOptions) *Thread {
	return newThread(c.runner, c.options, firstThreadOptions(options), "")
}

func (c *Codex) ResumeThread(id string, options ...ThreadOptions) *Thread {
	return newThread(c.runner, c.options, firstThreadOptions(options), id)
}

func firstThreadOptions(options []ThreadOptions) ThreadOptions {
	if len(options) == 0 {
		return ThreadOptions{}
	}
	return options[0]
}
