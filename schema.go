package codex

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
)

type outputSchemaFile struct {
	path    string
	cleanup func() error
}

func createOutputSchemaFile(schema any) (outputSchemaFile, error) {
	if schema == nil {
		return outputSchemaFile{cleanup: func() error { return nil }}, nil
	}
	if !isJSONObject(schema) {
		return outputSchemaFile{}, errors.New("outputSchema must be a plain JSON object")
	}

	dir, err := os.MkdirTemp("", "codex-output-schema-")
	if err != nil {
		return outputSchemaFile{}, err
	}
	cleanup := func() error {
		return os.RemoveAll(dir)
	}

	path := filepath.Join(dir, "schema.json")
	data, err := json.Marshal(schema)
	if err != nil {
		_ = cleanup()
		return outputSchemaFile{}, err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		_ = cleanup()
		return outputSchemaFile{}, err
	}
	return outputSchemaFile{path: path, cleanup: cleanup}, nil
}

func isJSONObject(value any) bool {
	if value == nil {
		return false
	}
	if _, ok := asStringMap(value); ok {
		return true
	}
	typ := reflect.TypeOf(value)
	switch typ.Kind() {
	case reflect.Map:
		return typ.Key().Kind() == reflect.String
	case reflect.Struct:
		return true
	default:
		return false
	}
}
