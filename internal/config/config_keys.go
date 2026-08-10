package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Get returns the effective value of a dotted config key (e.g.
// "defaults.project" or "projects.MERADIO.board_id") as a string. The value is
// taken from the loaded config after environment overlays and defaults are
// applied. Mid-level keys (e.g. "git") are printed as YAML.
func Get(path, envPath, key string) (string, error) {
	cfg, err := Load(path, envPath)
	if err != nil {
		return "", err
	}

	raw, err := marshalYAML(cfg)
	if err != nil {
		return "", fmt.Errorf("failed to encode config: %w", err)
	}
	var m map[string]any
	if err := yaml.Unmarshal([]byte(raw), &m); err != nil {
		return "", fmt.Errorf("failed to decode config: %w", err)
	}

	v, ok := lookupValue(m, key)
	if !ok {
		return "", fmt.Errorf("config key not found: %s", key)
	}
	return formatValue(v)
}

// Set writes value at the dotted key in the config file, creating the file and
// any intermediate mappings as needed. Existing comments are preserved. The
// value is stored as a typed YAML scalar (bool/int/float when the string
// parses as one, otherwise a string) so the config struct can decode it back.
// It returns whether the file was changed.
func Set(path, key, value string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("failed to read config %s: %w", path, err)
	}

	var doc yaml.Node
	if len(data) > 0 {
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return false, fmt.Errorf("failed to parse config %s: %w", path, err)
		}
	}
	mapping := ensureMapping(&doc)
	if mapping.Kind != yaml.MappingNode {
		return false, fmt.Errorf("config %s does not contain a YAML mapping", path)
	}

	valueNode := typedScalar(value)
	cur := mapping
	changed := false
	parts := strings.Split(key, ".")
	for i, part := range parts {
		last := i == len(parts)-1
		node := mappingKey(cur, part)
		if node == nil {
			child := valueNode
			if !last {
				child = &yaml.Node{Kind: yaml.MappingNode}
			}
			cur.Content = append(cur.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: part},
				child)
			cur = child
			changed = true
			continue
		}
		if last {
			if scalarsEqual(node, valueNode) {
				return false, nil
			}
			replacement := *valueNode
			replacement.HeadComment = node.HeadComment
			replacement.LineComment = node.LineComment
			replacement.FootComment = node.FootComment
			*node = replacement
			changed = true
			continue
		}
		if node.Kind != yaml.MappingNode {
			*node = yaml.Node{Kind: yaml.MappingNode}
			changed = true
		}
		cur = node
	}
	if !changed {
		return false, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("failed to create config dir: %w", err)
	}
	out, err := marshalNodeYAML(&doc)
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return false, fmt.Errorf("failed to write config %s: %w", path, err)
	}
	return true, nil
}

// Dump returns the full effective config as YAML with the Jira API token
// redacted (when one is set).
func Dump(path, envPath string) (string, error) {
	cfg, err := Load(path, envPath)
	if err != nil {
		return "", err
	}
	if cfg.Jira.APIToken != "" {
		cfg.Jira.APIToken = "REDACTED"
	}
	return marshalYAML(cfg)
}

func lookupValue(m map[string]any, key string) (any, bool) {
	var cur any = m
	parts := strings.Split(key, ".")
	for i, part := range parts {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		v, ok := mm[part]
		if !ok {
			return nil, false
		}
		if i == len(parts)-1 {
			return v, true
		}
		cur = v
	}
	return nil, false
}

func formatValue(v any) (string, error) {
	switch v.(type) {
	case string, bool, int, int64, float64, uint:
		return fmt.Sprint(v), nil
	case nil:
		return "", nil
	default:
		out, err := marshalYAML(v)
		if err != nil {
			return "", err
		}
		return strings.TrimRight(out, "\n"), nil
	}
}

// typedScalar builds a YAML scalar node whose tag matches the value, so the
// config struct can decode it back into the right Go type.
func typedScalar(s string) *yaml.Node {
	n := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s}
	if b, err := strconv.ParseBool(s); err == nil {
		n.Tag = "!!bool"
		n.Value = strconv.FormatBool(b)
		return n
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		n.Tag = "!!int"
		n.Value = strconv.FormatInt(i, 10)
		return n
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		n.Tag = "!!float"
		n.Value = strconv.FormatFloat(f, 'g', -1, 64)
		return n
	}
	return n
}

// mappingKey returns the value node for key in a mapping node, or nil.
func mappingKey(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// ensureMapping returns the mapping node under doc, creating a document +
// mapping structure when the node tree is empty.
func ensureMapping(doc *yaml.Node) *yaml.Node {
	if doc.Kind == 0 {
		doc.Kind = yaml.DocumentNode
	}
	if doc.Kind == yaml.DocumentNode {
		if len(doc.Content) == 0 {
			doc.Content = []*yaml.Node{{Kind: yaml.MappingNode}}
		}
		return doc.Content[0]
	}
	return doc
}

func scalarsEqual(a, b *yaml.Node) bool {
	return a.Kind == yaml.ScalarNode && b.Kind == yaml.ScalarNode &&
		a.Tag == b.Tag && a.Value == b.Value
}

func marshalYAML(v any) (string, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return "", fmt.Errorf("failed to encode YAML: %w", err)
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func marshalNodeYAML(n *yaml.Node) (string, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(n); err != nil {
		return "", fmt.Errorf("failed to encode config: %w", err)
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
	return buf.String(), nil
}
