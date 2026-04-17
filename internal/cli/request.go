package cli

import (
	"bytes"
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func readStdinYAML[T any](cmd *cobra.Command) (T, map[string]bool, error) {
	var zero T

	data, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return zero, nil, err
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	var target T
	if err := decoder.Decode(&target); err != nil {
		return zero, nil, invalidInputError(fmt.Sprintf("invalid stdin YAML: %v", err), commandState(cmd, nil))
	}

	node := yaml.Node{}
	if err := yaml.Unmarshal(data, &node); err != nil {
		return zero, nil, invalidInputError(fmt.Sprintf("invalid stdin YAML: %v", err), commandState(cmd, nil))
	}

	present := map[string]bool{}
	if len(node.Content) > 0 && node.Content[0].Kind == yaml.MappingNode {
		for i := 0; i+1 < len(node.Content[0].Content); i += 2 {
			present[node.Content[0].Content[i].Value] = true
		}
	}

	return target, present, nil
}

func itoa(value int) string {
	return strconv.Itoa(value)
}
