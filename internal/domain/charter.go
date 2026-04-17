package domain

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
)

const missingCharterGroupOrder = int(^uint(0) >> 1)

type CharterGroup struct {
	Key   string `yaml:"key" json:"key"`
	Title string `yaml:"title" json:"title"`
	Order int    `yaml:"order" json:"order"`
}

type CharterSpecEntry struct {
	Slug      string   `yaml:"slug" json:"slug"`
	Group     string   `yaml:"group" json:"group"`
	Order     int      `yaml:"order" json:"order"`
	DependsOn []string `yaml:"depends_on" json:"depends_on"`
	Notes     string   `yaml:"notes" json:"notes"`
}

type Charter struct {
	Name        string             `yaml:"name"`
	Title       string             `yaml:"title"`
	Description string             `yaml:"description"`
	Groups      []CharterGroup     `yaml:"groups"`
	Specs       []CharterSpecEntry `yaml:"specs"`
	DirPath     string             `yaml:"-"`
}

type LenientCharterOrdering struct {
	Specs []CharterSpecEntry
	Index map[string]int
}

type CharterCycleError struct {
	Slugs []string
}

func (e *CharterCycleError) Error() string {
	if len(e.Slugs) == 0 {
		return "charter dependency cycle detected"
	}
	return fmt.Sprintf("charter dependency cycle detected: %s", strings.Join(e.Slugs, ", "))
}

type CharterValidationError struct {
	Messages []string
}

func (e *CharterValidationError) Error() string {
	return strings.Join(e.Messages, "; ")
}

func newCharterValidationError(messages ...string) error {
	if len(messages) == 0 {
		return nil
	}
	return &CharterValidationError{Messages: slices.Clone(messages)}
}

func NewCharterSpecEntry(slug, group string, order int, dependsOn []string, notes string) (CharterSpecEntry, error) {
	entry := CharterSpecEntry{
		Slug:      slug,
		Group:     group,
		Order:     order,
		DependsOn: slices.Clone(dependsOn),
		Notes:     notes,
	}
	if err := validateCharterSpecEntry(entry); err != nil {
		return CharterSpecEntry{}, err
	}
	return entry, nil
}

func (c *Charter) GroupByKey(key string) *CharterGroup {
	for i := range c.Groups {
		if c.Groups[i].Key == key {
			return &c.Groups[i]
		}
	}
	return nil
}

func (c *Charter) SpecBySlug(slug string) *CharterSpecEntry {
	for i := range c.Specs {
		if c.Specs[i].Slug == slug {
			return &c.Specs[i]
		}
	}
	return nil
}

func (c *Charter) Validate() error {
	messages := make([]string, 0)
	add := func(message string) {
		messages = append(messages, message)
	}

	if !slugPattern.MatchString(c.Name) {
		add("name must match ^[a-z0-9][a-z0-9-]*$")
	}
	if strings.TrimSpace(c.Title) == "" {
		add("title is required")
	}
	if strings.TrimSpace(c.Description) == "" {
		add("description is required")
	}

	groupKeys := make(map[string]struct{}, len(c.Groups))
	for _, group := range c.Groups {
		if err := validateCharterGroup(group); err != nil {
			add(err.Error())
		}
		if _, exists := groupKeys[group.Key]; exists {
			add(fmt.Sprintf("duplicate group key %q", group.Key))
		}
		groupKeys[group.Key] = struct{}{}
	}

	specSlugs := make(map[string]struct{}, len(c.Specs))
	for _, spec := range c.Specs {
		if err := validateCharterSpecEntry(spec); err != nil {
			add(err.Error())
		}
		if _, exists := specSlugs[spec.Slug]; exists {
			add(fmt.Sprintf("duplicate spec slug %q", spec.Slug))
		}
		specSlugs[spec.Slug] = struct{}{}

		if _, exists := groupKeys[spec.Group]; !exists {
			add(fmt.Sprintf("spec %q references unknown group %q", spec.Slug, spec.Group))
		}
		seenDeps := map[string]struct{}{}
		for _, dependency := range spec.DependsOn {
			if dependency == spec.Slug {
				add(fmt.Sprintf("spec %q cannot depend on itself", spec.Slug))
			}
			if _, exists := seenDeps[dependency]; exists {
				add(fmt.Sprintf("spec %q has duplicate dependency %q", spec.Slug, dependency))
			}
			seenDeps[dependency] = struct{}{}
		}
	}

	for _, spec := range c.Specs {
		for _, dependency := range spec.DependsOn {
			if _, exists := specSlugs[dependency]; !exists {
				add(fmt.Sprintf("spec %q depends on unknown spec %q", spec.Slug, dependency))
			}
		}
	}

	if _, err := c.OrderedSpecs(); err != nil {
		var cycleErr *CharterCycleError
		if errors.As(err, &cycleErr) && len(messages) == 0 {
			return cycleErr
		}
		add(err.Error())
	}

	return newCharterValidationError(messages...)
}

func (c *Charter) EnsureGroup(group CharterGroup) error {
	if existing := c.GroupByKey(group.Key); existing != nil {
		return nil
	}
	if err := validateCharterGroup(group); err != nil {
		return err
	}

	c.Groups = append(c.Groups, group)
	if err := c.Validate(); err != nil {
		c.Groups = c.Groups[:len(c.Groups)-1]
		return err
	}
	return nil
}

func (c *Charter) ReplaceSpecEntry(entry CharterSpecEntry) error {
	if err := validateCharterSpecEntry(entry); err != nil {
		return err
	}

	updated := slices.Clone(c.Specs)
	replaced := false
	for i := range updated {
		if updated[i].Slug == entry.Slug {
			updated[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		updated = append(updated, entry)
	}

	previous := c.Specs
	c.Specs = updated
	if err := c.Validate(); err != nil {
		c.Specs = previous
		return err
	}
	return nil
}

func (c *Charter) MissingTrackingSpecs(trackingSlugs []string) []string {
	trackingSet := make(map[string]struct{}, len(trackingSlugs))
	for _, slug := range trackingSlugs {
		trackingSet[slug] = struct{}{}
	}

	missing := make([]string, 0)
	for _, spec := range c.Specs {
		if _, exists := trackingSet[spec.Slug]; !exists {
			missing = append(missing, spec.Slug)
		}
	}
	sort.Strings(missing)
	return missing
}

func (c *Charter) ExtraTrackingSpecs(trackingSlugs []string) []string {
	declared := make(map[string]struct{}, len(c.Specs))
	for _, spec := range c.Specs {
		declared[spec.Slug] = struct{}{}
	}

	extra := make([]string, 0)
	for _, slug := range trackingSlugs {
		if _, exists := declared[slug]; !exists {
			extra = append(extra, slug)
		}
	}
	sort.Strings(extra)
	return extra
}

func (c *Charter) OrderedSpecs() ([]CharterSpecEntry, error) {
	bySlug := make(map[string]CharterSpecEntry, len(c.Specs))
	groupOrder := charterGroupOrder(c)
	for _, spec := range c.Specs {
		bySlug[spec.Slug] = spec
	}

	inDegree := make(map[string]int, len(c.Specs))
	graph := make(map[string][]string, len(c.Specs))
	for _, spec := range c.Specs {
		inDegree[spec.Slug] = 0
		for _, dependency := range spec.DependsOn {
			if _, exists := bySlug[dependency]; !exists {
				continue
			}
			inDegree[spec.Slug]++
			graph[dependency] = append(graph[dependency], spec.Slug)
		}
	}

	queue := make([]CharterSpecEntry, 0, len(c.Specs))
	for _, spec := range c.Specs {
		if inDegree[spec.Slug] == 0 {
			queue = append(queue, spec)
		}
	}
	orderCharterSpecEntries(queue, groupOrder)

	ordered := make([]CharterSpecEntry, 0, len(c.Specs))
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		ordered = append(ordered, current)

		for _, dependent := range graph[current.Slug] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, bySlug[dependent])
			}
		}
		orderCharterSpecEntries(queue, groupOrder)
	}

	if len(ordered) != len(c.Specs) {
		blocked := make([]CharterSpecEntry, 0, len(c.Specs)-len(ordered))
		for _, spec := range c.Specs {
			if inDegree[spec.Slug] > 0 {
				blocked = append(blocked, spec)
			}
		}
		orderCharterSpecEntries(blocked, groupOrder)
		return nil, &CharterCycleError{Slugs: findCharterCycleSlugs(blocked, bySlug, groupOrder)}
	}

	return ordered, nil
}

func OrderedCharterSpecsLenient(charter *Charter) []CharterSpecEntry {
	return BuildLenientCharterOrdering(charter).Specs
}

func BuildLenientCharterOrdering(charter *Charter) LenientCharterOrdering {
	if charter == nil {
		return LenientCharterOrdering{
			Specs: []CharterSpecEntry{},
			Index: map[string]int{},
		}
	}
	ordered, err := charter.OrderedSpecs()
	if err != nil {
		ordered = append([]CharterSpecEntry{}, charter.Specs...)
		orderCharterSpecEntries(ordered, charterGroupOrder(charter))
	}
	index := make(map[string]int, len(ordered))
	for i, entry := range ordered {
		index[entry.Slug] = i
	}
	return LenientCharterOrdering{
		Specs: ordered,
		Index: index,
	}
}

func LenientCharterOrder(charter *Charter) map[string]int {
	return BuildLenientCharterOrdering(charter).Index
}

func charterGroupOrder(charter *Charter) map[string]int {
	groupOrder := make(map[string]int, len(charter.Groups))
	for _, group := range charter.Groups {
		groupOrder[group.Key] = group.Order
	}
	return groupOrder
}

func orderCharterSpecEntries(entries []CharterSpecEntry, groupOrder map[string]int) {
	sort.Slice(entries, func(i, j int) bool {
		left := entries[i]
		right := entries[j]
		leftGroupOrder, leftExists := groupOrder[left.Group]
		rightGroupOrder, rightExists := groupOrder[right.Group]
		if !leftExists {
			leftGroupOrder = missingCharterGroupOrder
		}
		if !rightExists {
			rightGroupOrder = missingCharterGroupOrder
		}
		if leftGroupOrder != rightGroupOrder {
			return leftGroupOrder < rightGroupOrder
		}
		if left.Order != right.Order {
			return left.Order < right.Order
		}
		return left.Slug < right.Slug
	})
}

func findCharterCycleSlugs(blocked []CharterSpecEntry, bySlug map[string]CharterSpecEntry, groupOrder map[string]int) []string {
	if len(blocked) == 0 {
		return nil
	}

	blockedSet := make(map[string]struct{}, len(blocked))
	for _, spec := range blocked {
		blockedSet[spec.Slug] = struct{}{}
	}

	dependencies := make(map[string][]CharterSpecEntry, len(blocked))
	for _, spec := range blocked {
		deps := make([]CharterSpecEntry, 0, len(spec.DependsOn))
		for _, dependency := range spec.DependsOn {
			dependencySpec, exists := bySlug[dependency]
			if !exists {
				continue
			}
			if _, blockedDependency := blockedSet[dependency]; !blockedDependency {
				continue
			}
			deps = append(deps, dependencySpec)
		}
		orderCharterSpecEntries(deps, groupOrder)
		dependencies[spec.Slug] = deps
	}

	const (
		unvisited = iota
		visiting
		visited
	)
	state := make(map[string]int, len(blocked))
	stack := make([]string, 0, len(blocked))
	stackIndex := make(map[string]int, len(blocked))
	cycle := []string{}

	var visit func(string) bool
	visit = func(slug string) bool {
		state[slug] = visiting
		stackIndex[slug] = len(stack)
		stack = append(stack, slug)

		for _, dependency := range dependencies[slug] {
			switch state[dependency.Slug] {
			case unvisited:
				if visit(dependency.Slug) {
					return true
				}
			case visiting:
				cycle = append([]string{}, stack[stackIndex[dependency.Slug]:]...)
				return true
			}
		}

		delete(stackIndex, slug)
		stack = stack[:len(stack)-1]
		state[slug] = visited
		return false
	}

	for _, spec := range blocked {
		if state[spec.Slug] == unvisited && visit(spec.Slug) {
			return cycle
		}
	}

	fallback := make([]string, 0, len(blocked))
	for _, spec := range blocked {
		fallback = append(fallback, spec.Slug)
	}
	return fallback
}

func validateCharterGroup(group CharterGroup) error {
	if !slugPattern.MatchString(group.Key) {
		return newCharterValidationError(fmt.Sprintf("group key %q must match ^[a-z0-9][a-z0-9-]*$", group.Key))
	}
	if strings.TrimSpace(group.Title) == "" {
		return newCharterValidationError(fmt.Sprintf("group %q title is required", group.Key))
	}
	if group.Order < 0 {
		return newCharterValidationError(fmt.Sprintf("group %q order must be >= 0", group.Key))
	}
	return nil
}

func validateCharterSpecEntry(spec CharterSpecEntry) error {
	if !slugPattern.MatchString(spec.Slug) {
		return newCharterValidationError(fmt.Sprintf("spec slug %q must match ^[a-z0-9][a-z0-9-]*$", spec.Slug))
	}
	if !slugPattern.MatchString(spec.Group) {
		return newCharterValidationError(fmt.Sprintf("spec %q group %q must match ^[a-z0-9][a-z0-9-]*$", spec.Slug, spec.Group))
	}
	if spec.Order < 0 {
		return newCharterValidationError(fmt.Sprintf("spec %q order must be >= 0", spec.Slug))
	}
	if strings.TrimSpace(spec.Notes) == "" {
		return newCharterValidationError(fmt.Sprintf("spec %q notes are required", spec.Slug))
	}
	return nil
}
