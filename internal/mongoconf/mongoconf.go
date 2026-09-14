// Package mongoconf reads and writes mongod configuration files while
// preserving comments, key order and formatting of everything it does not touch.
package mongoconf

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Doc is a parsed mongod.conf document.
type Doc struct {
	root *yaml.Node
}

// New returns an empty configuration document.
func New() *Doc {
	return &Doc{
		root: &yaml.Node{
			Kind: yaml.DocumentNode,
			Content: []*yaml.Node{
				{Kind: yaml.MappingNode, Tag: "!!map"},
			},
		},
	}
}

// Parse reads a configuration document. Empty input yields an empty document.
func Parse(data []byte) (*Doc, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return New(), nil
	}

	root := &yaml.Node{}
	if err := yaml.Unmarshal(data, root); err != nil {
		return nil, fmt.Errorf("mongoconf: parse: %w", err)
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return nil, errors.New("mongoconf: document is not a YAML mapping")
	}
	if root.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("mongoconf: document root is not a mapping")
	}
	return &Doc{root: root}, nil
}

// Bytes renders the document back to YAML with two-space indentation.
func (d *Doc) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(d.root); err != nil {
		return nil, fmt.Errorf("mongoconf: render: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("mongoconf: render: %w", err)
	}
	return buf.Bytes(), nil
}

// String renders the document, returning the error text on failure so the value
// is always usable in logs and UI previews.
func (d *Doc) String() string {
	data, err := d.Bytes()
	if err != nil {
		return fmt.Sprintf("<invalid config: %v>", err)
	}
	return string(data)
}

// Set writes value at the given key path, creating intermediate mappings.
// A line comment already attached to the key is kept.
func (d *Doc) Set(value any, path ...string) error {
	if len(path) == 0 {
		return errors.New("mongoconf: empty key path")
	}

	mapping := d.mapping()
	for _, key := range path[:len(path)-1] {
		child := childValue(mapping, key)
		if child == nil {
			child = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			appendPair(mapping, key, child)
		} else if child.Kind != yaml.MappingNode {
			*child = yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		}
		mapping = child
	}

	encoded := &yaml.Node{}
	if err := encoded.Encode(value); err != nil {
		return fmt.Errorf("mongoconf: encode value for %q: %w", strings.Join(path, "."), err)
	}

	leaf := path[len(path)-1]
	if existing := childValue(mapping, leaf); existing != nil {
		comment := existing.LineComment
		*existing = *encoded
		existing.LineComment = comment
		return nil
	}
	appendPair(mapping, leaf, encoded)
	return nil
}

// Remove deletes the key at the given path. Missing keys are ignored.
func (d *Doc) Remove(path ...string) {
	if len(path) == 0 {
		return
	}

	mapping := d.mapping()
	for _, key := range path[:len(path)-1] {
		child := childValue(mapping, key)
		if child == nil || child.Kind != yaml.MappingNode {
			return
		}
		mapping = child
	}

	leaf := path[len(path)-1]
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == leaf {
			mapping.Content = append(mapping.Content[:i], mapping.Content[i+2:]...)
			return
		}
	}
}

// Lookup decodes the value at the given path into target.
func (d *Doc) Lookup(target any, path ...string) bool {
	node := d.node(path...)
	if node == nil {
		return false
	}
	return node.Decode(target) == nil
}

// GetString returns the string at path, or "" when absent.
func (d *Doc) GetString(path ...string) string {
	var value string
	d.Lookup(&value, path...)
	return value
}

// GetInt returns the int at path, or 0 when absent.
func (d *Doc) GetInt(path ...string) int {
	var value int
	d.Lookup(&value, path...)
	return value
}

// ToMap decodes the whole document into a plain map for transfer to the UI.
func (d *Doc) ToMap() (map[string]any, error) {
	result := map[string]any{}
	if err := d.mapping().Decode(&result); err != nil {
		return nil, fmt.Errorf("mongoconf: decode document: %w", err)
	}
	return result, nil
}

func (d *Doc) node(path ...string) *yaml.Node {
	current := d.mapping()
	for i, key := range path {
		child := childValue(current, key)
		if child == nil {
			return nil
		}
		if i == len(path)-1 {
			return child
		}
		if child.Kind != yaml.MappingNode {
			return nil
		}
		current = child
	}
	return nil
}

func (d *Doc) mapping() *yaml.Node {
	if len(d.root.Content) == 0 {
		d.root.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	}
	return d.root.Content[0]
}

func childValue(mapping *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

func appendPair(mapping *yaml.Node, key string, value *yaml.Node) {
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		value,
	)
}
