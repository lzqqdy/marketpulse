package admin

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/lzqqdy/marketpulse/internal/config"
)

// Gap is a structural difference between live and example config trees.
type Gap struct {
	Path                 string `json:"path"`
	Kind                 string `json:"kind"` // missing_in_live | missing_in_example | type_mismatch
	ExampleValue         any    `json:"exampleValue,omitempty"`
	LiveValue            any    `json:"liveValue,omitempty"`
	SuggestedYAMLSnippet string `json:"suggestedYamlSnippet,omitempty"`
}

// ConfigView is the GET /admin/config payload.
type ConfigView struct {
	Path                string         `json:"path"`
	ExamplePath         string         `json:"examplePath"`
	YAML                string         `json:"yaml"`
	ExampleYAML         string         `json:"exampleYaml"`
	Gaps                []Gap          `json:"gaps"`
	Schema              []FieldSchema  `json:"schema"`
	Sections            []SectionMeta  `json:"sections"`
	RestartRequiredHint bool           `json:"restartRequiredHint"`
}

// GetConfig loads live + example, computes gaps and schema.
func (s *Service) GetConfig() (ConfigView, error) {
	liveYAML, err := readFileOptional(s.configPath)
	if err != nil {
		return ConfigView{}, err
	}
	exampleYAML, err := readFileOptional(s.examplePath)
	if err != nil {
		return ConfigView{}, err
	}

	liveTree, err := parseYAMLTree([]byte(liveYAML))
	if err != nil {
		return ConfigView{}, fmt.Errorf("parse live config: %w", err)
	}
	exampleTree, err := parseYAMLTree([]byte(exampleYAML))
	if err != nil {
		return ConfigView{}, fmt.Errorf("parse example config: %w", err)
	}

	gaps := DiffTrees(liveTree, exampleTree)
	schema := BuildSchema(exampleTree, exampleYAML)

	return ConfigView{
		Path:                s.configPath,
		ExamplePath:         s.examplePath,
		YAML:                liveYAML,
		ExampleYAML:         exampleYAML,
		Gaps:                gaps,
		Schema:              schema,
		Sections:            DefaultSections(),
		RestartRequiredHint: true,
	}, nil
}

// SaveConfig validates and atomically writes live config.yaml.
func (s *Service) SaveConfig(raw string) error {
	data := []byte(raw)
	if _, err := config.ParseYAML(data); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	// Ensure it also parses as a generic tree (reject empty / scalar root).
	if _, err := parseYAMLTree(data); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	return atomicWriteFile(s.configPath, data)
}

func parseYAMLTree(data []byte) (map[string]any, error) {
	var root any
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(false)
	if err := dec.Decode(&root); err != nil {
		return nil, err
	}
	m, ok := root.(map[string]any)
	if !ok {
		// yaml.v3 may decode to map[string]interface{} already; also handle map[any]any
		if converted, ok := asStringKeyMap(root); ok {
			return converted, nil
		}
		return nil, fmt.Errorf("config root must be a mapping")
	}
	return normalizeMap(m), nil
}

func asStringKeyMap(v any) (map[string]any, bool) {
	switch m := v.(type) {
	case map[string]any:
		return normalizeMap(m), true
	case map[any]any:
		out := make(map[string]any, len(m))
		for k, val := range m {
			out[fmt.Sprint(k)] = val
		}
		return normalizeMap(out), true
	default:
		return nil, false
	}
}

func normalizeMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = normalizeValue(v)
	}
	return out
}

func normalizeValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return normalizeMap(t)
	case map[any]any:
		m := make(map[string]any, len(t))
		for k, val := range t {
			m[fmt.Sprint(k)] = normalizeValue(val)
		}
		return m
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = normalizeValue(item)
		}
		return out
	default:
		return v
	}
}

func atomicWriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
