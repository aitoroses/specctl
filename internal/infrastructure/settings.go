package infrastructure

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Hook represents a single hook entry in settings.json.
type Hook struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

// HookGroup represents a group of hooks with a matcher.
type HookGroup struct {
	Matcher string `json:"matcher"`
	Hooks   []Hook `json:"hooks"`
}

func loadHookSettings(path string) (map[string]interface{}, map[string][]HookGroup, error) {
	var raw map[string]interface{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			raw = make(map[string]interface{})
		} else {
			return nil, nil, fmt.Errorf("reading %s: %w", path, err)
		}
	} else {
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.UseNumber()
		if err := dec.Decode(&raw); err != nil {
			return nil, nil, fmt.Errorf("parsing %s: %w", path, err)
		}
	}

	hooksMap := make(map[string][]HookGroup)
	if hooksRaw, ok := raw["hooks"]; ok {
		b, _ := json.Marshal(hooksRaw)
		if err := json.Unmarshal(b, &hooksMap); err != nil {
			return nil, nil, fmt.Errorf("parsing hooks: %w", err)
		}
	}

	return raw, hooksMap, nil
}

func writeHookSettings(path string, raw map[string]interface{}, hooksMap map[string][]HookGroup) error {
	raw["hooks"] = hooksMap

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(raw); err != nil {
		return fmt.Errorf("serializing settings: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}

// EnsureHook adds a hook to settings.json if it doesn't already exist.
// Returns (true, nil) if installed, (false, nil) if already present.
func EnsureHook(path, event, matcher, command string, timeout int) (bool, error) {
	raw, hooksMap, err := loadHookSettings(path)
	if err != nil {
		return false, err
	}

	// Check if our hook already exists, update if stale
	groups := hooksMap[event]
	for i, g := range groups {
		if g.Matcher == matcher {
			for j, h := range g.Hooks {
				if strings.Contains(h.Command, "specctl hook") {
					if h.Command == command {
						return false, nil // already up to date
					}
					// Update stale hook command
					groups[i].Hooks[j].Command = command
					groups[i].Hooks[j].Timeout = timeout
					hooksMap[event] = groups
					goto write
				}
			}
		}
	}

	// Not found — add new hook
	{
		found := false
		for i, g := range groups {
			if g.Matcher == matcher {
				groups[i].Hooks = append(groups[i].Hooks, Hook{
					Type:    "command",
					Command: command,
					Timeout: timeout,
				})
				found = true
				break
			}
		}
		if !found {
			groups = append(groups, HookGroup{
				Matcher: matcher,
				Hooks: []Hook{{
					Type:    "command",
					Command: command,
					Timeout: timeout,
				}},
			})
		}
		hooksMap[event] = groups
	}

write:
	if err := writeHookSettings(path, raw, hooksMap); err != nil {
		return false, err
	}

	return true, nil
}

// RemoveHooksContaining removes hooks for an event+matcher whose command contains the given substring.
// Returns true if any hooks were removed.
func RemoveHooksContaining(path, event, matcher, commandSubstring string) (bool, error) {
	raw, hooksMap, err := loadHookSettings(path)
	if err != nil {
		return false, err
	}

	groups := hooksMap[event]
	if len(groups) == 0 {
		return false, nil
	}

	removed := false
	filteredGroups := make([]HookGroup, 0, len(groups))
	for _, g := range groups {
		if g.Matcher != matcher {
			filteredGroups = append(filteredGroups, g)
			continue
		}

		filteredHooks := make([]Hook, 0, len(g.Hooks))
		for _, h := range g.Hooks {
			if strings.Contains(h.Command, commandSubstring) {
				removed = true
				continue
			}
			filteredHooks = append(filteredHooks, h)
		}

		if len(filteredHooks) > 0 {
			g.Hooks = filteredHooks
			filteredGroups = append(filteredGroups, g)
		}
	}
	hooksMap[event] = filteredGroups

	if !removed {
		return false, nil
	}
	if err := writeHookSettings(path, raw, hooksMap); err != nil {
		return false, err
	}
	return true, nil
}
