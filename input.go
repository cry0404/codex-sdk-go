package codex

type UserInput struct {
	Type string
	Text string
	Path string
}

type Input struct {
	prompt string
	items  []UserInput
}

func TextInput(text string) Input {
	return Input{prompt: text}
}

func StructuredInput(items ...UserInput) Input {
	return Input{items: append([]UserInput(nil), items...)}
}

func Text(text string) UserInput {
	return UserInput{Type: "text", Text: text}
}

func LocalImage(path string) UserInput {
	return UserInput{Type: "local_image", Path: path}
}
