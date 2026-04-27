package mcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aitoroses/specctl/internal/application"
	"github.com/aitoroses/specctl/internal/domain"
	"github.com/aitoroses/specctl/internal/presenter"
)

// panicLogger captures panic stderr output. Defaults to os.Stderr but
// tests swap it via SetPanicLogger to assert on the captured output.
var panicLogger io.Writer = os.Stderr

// SetPanicLogger replaces the destination for recovered-panic stack traces.
// It is intended for tests; callers must restore the previous value.
func SetPanicLogger(w io.Writer) (restore func()) {
	prev := panicLogger
	if w == nil {
		w = os.Stderr
	}
	panicLogger = w
	return func() { panicLogger = prev }
}

type Server struct {
	service *application.Service
	server  *sdk.Server
}

func RunStdio(ctx context.Context) error {
	service, err := application.OpenFromWorkingDir()
	if err != nil {
		// If no .specs/ directory found, start the MCP server anyway
		// with a nil service. Tools will return guidance to initialize.
		s := NewUninitializedServer()
		runErr := s.Run(ctx)
		if runErr == nil || errors.Is(runErr, io.EOF) {
			return nil
		}
		return runErr
	}
	err = NewServer(service).Run(ctx)
	if err == nil || errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func NewServer(service *application.Service) *Server {
	s := &Server{
		service: service,
		server: sdk.NewServer(&sdk.Implementation{
			Name:    "specctl",
			Version: "v1",
		}, nil),
	}
	s.registerTools()
	return s
}

// NewUninitializedServer creates a server that responds to all tools
// with guidance to initialize specctl. Used when no .specs/ directory
// exists in the repo.
func NewUninitializedServer() *Server {
	s := &Server{
		server: sdk.NewServer(&sdk.Implementation{
			Name:    "specctl",
			Version: "v1",
		}, nil),
	}
	s.registerUninitializedTools()
	return s
}

func (s *Server) registerUninitializedTools() {
	uninit := func(ctx context.Context, req *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, any, error) {
		envelope := presenter.Envelope{
			State: map[string]any{"initialized": false},
			Focus: map[string]any{"reason": "specctl is not initialized in this repository"},
			Next: presenter.Directive{
				Mode: "sequence",
				Steps: []any{
					map[string]any{
						"action":      "initialize",
						"description": "Initialize specctl governance in this repository",
						"mcp":         map[string]any{"available": true, "tool": "specctl_init", "input": map[string]any{}},
					},
				},
			},
			Error: &presenter.Error{
				Code:    "NOT_INITIALIZED",
				Message: "No .specs/ directory found. Call specctl_init to initialize specctl governance in this repository.",
			},
		}
		return s.toolResult(envelope), nil, nil
	}

	// specctl_init is the one tool that works without initialization.
	addTool(s, &sdk.Tool{Name: "specctl_init", Description: "Initialize specctl governance in this repository."}, s.handleInit)

	// All other tools return NOT_INITIALIZED with guidance to call specctl_init.
	toolNames := []struct{ name, desc string }{
		{"specctl_context", "Read registry, charter, spec, or file ownership context."},
		{"specctl_diff", "Return a semantic diff against the stored checkpoint."},
		{"specctl_charter_create", "Create a charter directory and CHARTER.yaml."},
		{"specctl_spec_create", "Create a tracking file and charter membership."},
		{"specctl_delta_add", "Add a tracked delta."},
		{"specctl_delta_start", "Move open delta to in-progress."},
		{"specctl_delta_defer", "Move open or in-progress delta to deferred."},
		{"specctl_delta_resume", "Move deferred delta back to open."},
		{"specctl_delta_close", "Close a delta when its obligations are satisfied."},
		{"specctl_delta_withdraw", "Retract an open, in-progress, or deferred delta with an auditable reason."},
		{"specctl_delta_rebind_requirements", "Rebind affects_requirements of an open, in-progress, or deferred delta."},
		{"specctl_requirement_add", "Add a tracked requirement from Gherkin."},
		{"specctl_requirement_replace", "Replace one tracked requirement with a new active requirement."},
		{"specctl_requirement_refresh", "Refresh the stored requirement block in place."},
		{"specctl_requirement_withdraw", "Withdraw one active requirement."},
		{"specctl_requirement_stale", "Mark one active requirement stale."},
		{"specctl_requirement_verify", "Mark one requirement as verified."},
		{"specctl_revision_bump", "Advance rev, checkpoint, and changelog."},
		{"specctl_sync", "Re-anchor checkpoint drift without bumping rev."},
		{"specctl_doc_add", "Add a secondary document reference to a spec."},
		{"specctl_doc_remove", "Remove a secondary document reference from a spec."},
		{"specctl_config", "Read project config (gherkin tags, source prefixes)."},
		{"specctl_config_add_tag", "Register a gherkin tag so @tag is accepted in requirement blocks."},
		{"specctl_config_remove_tag", "Remove a gherkin tag if no requirement uses it."},
		{"specctl_config_add_prefix", "Add one source prefix if the directory exists."},
		{"specctl_config_remove_prefix", "Remove one source prefix."},
		{"specctl_charter_add_spec", "Create or replace one charter membership entry."},
		{"specctl_charter_remove_spec", "Remove one charter membership entry without deleting the tracking file."},
	}
	for _, t := range toolNames {
		addTool(s, &sdk.Tool{Name: t.name, Description: t.desc}, uninit)
	}
}

func (s *Server) Run(ctx context.Context) error {
	return s.server.Run(ctx, &sdk.StdioTransport{})
}

type contextInput struct {
	Target string `json:"target,omitempty" jsonschema:"spec target or charter name"`
	File   string `json:"file,omitempty" jsonschema:"repo-relative path for ownership resolution"`
}

type diffInput struct {
	Target  string `json:"target,omitempty" jsonschema:"spec target"`
	Charter string `json:"charter,omitempty" jsonschema:"charter name"`
}

type charterGroupInput struct {
	Key   string `json:"key" jsonschema:"charter group key"`
	Title string `json:"title" jsonschema:"charter group title"`
	Order int    `json:"order" jsonschema:"charter group order"`
}

type charterCreateInput struct {
	Charter     string              `json:"charter" jsonschema:"charter name"`
	Title       string              `json:"title" jsonschema:"charter title"`
	Description string              `json:"description" jsonschema:"charter description"`
	Groups      []charterGroupInput `json:"groups,omitempty" jsonschema:"optional charter groups"`
}

type specCreateInput struct {
	Spec         string   `json:"spec" jsonschema:"spec identifier charter:slug"`
	Title        string   `json:"title" jsonschema:"human-readable spec title"`
	Doc          string   `json:"doc" jsonschema:"repo-relative primary design document path"`
	Scope        []string `json:"scope" jsonschema:"repo-relative governed directories"`
	Group        string   `json:"group" jsonschema:"charter group key"`
	GroupTitle   *string  `json:"group_title,omitempty" jsonschema:"optional new group title"`
	GroupOrder   *int     `json:"group_order,omitempty" jsonschema:"optional new group order"`
	Order        int      `json:"order" jsonschema:"order inside the charter group"`
	CharterNotes string   `json:"charter_notes" jsonschema:"charter planning note"`
	DependsOn    []string `json:"depends_on,omitempty" jsonschema:"dependency slugs in the same charter"`
	Tags         []string `json:"tags,omitempty" jsonschema:"informational spec tags"`
}

type deltaAddInput struct {
	Spec                string   `json:"spec" jsonschema:"spec identifier charter:slug"`
	Intent              string   `json:"intent" jsonschema:"delta intent: add, change, remove, repair"`
	Area                string   `json:"area" jsonschema:"human-readable gap area"`
	Current             string   `json:"current" jsonschema:"current observed state"`
	Desired             string   `json:"desired" jsonschema:"target state"`
	Notes               string   `json:"notes" jsonschema:"why this delta exists"`
	AffectsRequirements []string `json:"affects_requirements,omitempty" jsonschema:"active requirement IDs affected by this delta"`
}

type deltaTransitionInput struct {
	Spec    string `json:"spec" jsonschema:"spec identifier charter:slug"`
	DeltaID string `json:"delta_id" jsonschema:"delta ID"`
}

type deltaWithdrawInput struct {
	Spec    string `json:"spec" jsonschema:"spec identifier charter:slug"`
	DeltaID string `json:"delta_id" jsonschema:"delta ID to retract"`
	Reason  string `json:"reason" jsonschema:"auditable reason recorded with the withdrawal"`
}

type deltaRebindInput struct {
	Spec    string `json:"spec" jsonschema:"spec identifier charter:slug"`
	DeltaID string `json:"delta_id" jsonschema:"delta ID whose affects_requirements is rebound"`
	From    string `json:"from" jsonschema:"requirement ID currently in affects_requirements"`
	To      string `json:"to,omitempty" jsonschema:"replacement requirement ID; required unless remove is true"`
	Remove  bool   `json:"remove,omitempty" jsonschema:"drop the from anchor instead of rebinding; requires reason"`
	Reason  string `json:"reason,omitempty" jsonschema:"auditable reason; required when remove is true, optional on rebinds"`
}

type requirementAddInput struct {
	Spec    string `json:"spec" jsonschema:"spec identifier charter:slug"`
	DeltaID string `json:"delta_id" jsonschema:"introducing delta ID"`
	Gherkin string `json:"gherkin" jsonschema:"requirement-level gherkin block"`
}

type requirementReplaceInput struct {
	Spec          string `json:"spec" jsonschema:"spec identifier charter:slug"`
	RequirementID string `json:"requirement_id" jsonschema:"requirement ID"`
	DeltaID       string `json:"delta_id" jsonschema:"introducing delta ID"`
	Gherkin       string `json:"gherkin" jsonschema:"replacement requirement gherkin block"`
}

type requirementRefreshInput struct {
	Spec          string `json:"spec" jsonschema:"spec identifier charter:slug"`
	RequirementID string `json:"requirement_id" jsonschema:"requirement ID"`
	Gherkin       string `json:"gherkin" jsonschema:"refreshed requirement gherkin block"`
}

type requirementDeltaInput struct {
	Spec          string `json:"spec" jsonschema:"spec identifier charter:slug"`
	RequirementID string `json:"requirement_id" jsonschema:"requirement ID"`
	DeltaID       string `json:"delta_id" jsonschema:"delta ID"`
}

type requirementVerifyInput struct {
	Spec          string   `json:"spec" jsonschema:"spec identifier charter:slug"`
	RequirementID string   `json:"requirement_id" jsonschema:"requirement ID"`
	TestFiles     []string `json:"test_files,omitempty" jsonschema:"repo-relative verification files"`
}

type revisionBumpInput struct {
	Spec       string `json:"spec" jsonschema:"spec identifier charter:slug"`
	Checkpoint string `json:"checkpoint" jsonschema:"git commit or ref"`
	Summary    string `json:"summary" jsonschema:"changelog summary"`
}

type syncInput struct {
	Spec       string `json:"spec" jsonschema:"spec identifier charter:slug"`
	Checkpoint string `json:"checkpoint" jsonschema:"git commit or ref"`
	Summary    string `json:"summary" jsonschema:"sync summary"`
}

type docAddInput struct {
	Spec string `json:"spec" jsonschema:"spec identifier charter:slug"`
	Doc  string `json:"doc" jsonschema:"repo-relative path to secondary document"`
}

type docRemoveInput struct {
	Spec string `json:"spec" jsonschema:"spec identifier charter:slug"`
	Doc  string `json:"doc" jsonschema:"repo-relative path to secondary document"`
}

type configTagInput struct {
	Tag string `json:"tag" jsonschema:"gherkin tag to register or remove (without leading @)"`
}

type configPrefixInput struct {
	Prefix string `json:"prefix" jsonschema:"repo-relative source prefix path"`
}

type charterAddSpecInput struct {
	Charter    string   `json:"charter" jsonschema:"charter name"`
	Slug       string   `json:"slug" jsonschema:"spec slug"`
	Group      string   `json:"group" jsonschema:"charter group key"`
	GroupTitle string   `json:"group_title,omitempty" jsonschema:"optional new group title"`
	GroupOrder *int     `json:"group_order,omitempty" jsonschema:"optional new group order"`
	Order      int      `json:"order" jsonschema:"order inside the charter group"`
	DependsOn  []string `json:"depends_on,omitempty" jsonschema:"dependency slugs in the same charter"`
	Notes      string   `json:"notes" jsonschema:"charter planning note"`
}

type charterRemoveSpecInput struct {
	Charter string `json:"charter" jsonschema:"charter name"`
	Slug    string `json:"slug" jsonschema:"spec slug"`
}

func (s *Server) registerTools() {
	addTool(s, &sdk.Tool{Name: "specctl_init", Description: "Initialize specctl governance in this repository."}, s.handleInit)
	addTool(s, &sdk.Tool{Name: "specctl_context", Description: "Read registry, charter, spec, or file ownership context."}, s.handleContext)
	addTool(s, &sdk.Tool{Name: "specctl_diff", Description: "Return a semantic diff against the stored checkpoint."}, s.handleDiff)
	addTool(s, &sdk.Tool{Name: "specctl_charter_create", Description: "Create a charter directory and CHARTER.yaml."}, s.handleCharterCreate)
	addTool(s, &sdk.Tool{Name: "specctl_spec_create", Description: "Create a tracking file and charter membership."}, s.handleSpecCreate)
	addTool(s, &sdk.Tool{Name: "specctl_delta_add", Description: "Add a tracked delta."}, s.handleDeltaAdd)
	addTool(s, &sdk.Tool{Name: "specctl_delta_start", Description: "Move open delta to in-progress."}, s.handleDeltaStart)
	addTool(s, &sdk.Tool{Name: "specctl_delta_defer", Description: "Move open or in-progress delta to deferred."}, s.handleDeltaDefer)
	addTool(s, &sdk.Tool{Name: "specctl_delta_resume", Description: "Move deferred delta back to open."}, s.handleDeltaResume)
	addTool(s, &sdk.Tool{Name: "specctl_delta_close", Description: "Close a delta when its obligations are satisfied."}, s.handleDeltaClose)
	addTool(s, &sdk.Tool{Name: "specctl_delta_withdraw", Description: "Retract an open, in-progress, or deferred delta with an auditable reason."}, s.handleDeltaWithdraw)
	addTool(s, &sdk.Tool{Name: "specctl_delta_rebind_requirements", Description: "Rebind affects_requirements of an open, in-progress, or deferred delta."}, s.handleDeltaRebind)
	addTool(s, &sdk.Tool{Name: "specctl_requirement_add", Description: "Add a tracked requirement from Gherkin."}, s.handleRequirementAdd)
	addTool(s, &sdk.Tool{Name: "specctl_requirement_replace", Description: "Replace one tracked requirement with a new active requirement."}, s.handleRequirementReplace)
	addTool(s, &sdk.Tool{Name: "specctl_requirement_refresh", Description: "Refresh the stored requirement block in place."}, s.handleRequirementRefresh)
	addTool(s, &sdk.Tool{Name: "specctl_requirement_withdraw", Description: "Withdraw one active requirement."}, s.handleRequirementWithdraw)
	addTool(s, &sdk.Tool{Name: "specctl_requirement_stale", Description: "Mark one active requirement stale."}, s.handleRequirementStale)
	addTool(s, &sdk.Tool{Name: "specctl_requirement_verify", Description: "Mark one requirement as verified."}, s.handleRequirementVerify)
	addTool(s, &sdk.Tool{Name: "specctl_revision_bump", Description: "Advance rev, checkpoint, and changelog."}, s.handleRevisionBump)
	addTool(s, &sdk.Tool{Name: "specctl_sync", Description: "Re-anchor checkpoint drift without bumping rev."}, s.handleSync)
	addTool(s, &sdk.Tool{Name: "specctl_doc_add", Description: "Add a secondary document reference to a spec."}, s.handleDocAdd)
	addTool(s, &sdk.Tool{Name: "specctl_doc_remove", Description: "Remove a secondary document reference from a spec."}, s.handleDocRemove)
	addTool(s, &sdk.Tool{Name: "specctl_config", Description: "Read project config (gherkin tags, source prefixes)."}, s.handleConfig)
	addTool(s, &sdk.Tool{Name: "specctl_config_add_tag", Description: "Register a gherkin tag so @tag is accepted in requirement blocks."}, s.handleConfigAddTag)
	addTool(s, &sdk.Tool{Name: "specctl_config_remove_tag", Description: "Remove a gherkin tag if no requirement uses it."}, s.handleConfigRemoveTag)
	addTool(s, &sdk.Tool{Name: "specctl_config_add_prefix", Description: "Add one source prefix if the directory exists."}, s.handleConfigAddPrefix)
	addTool(s, &sdk.Tool{Name: "specctl_config_remove_prefix", Description: "Remove one source prefix."}, s.handleConfigRemovePrefix)
	addTool(s, &sdk.Tool{Name: "specctl_charter_add_spec", Description: "Create or replace one charter membership entry."}, s.handleCharterAddSpec)
	addTool(s, &sdk.Tool{Name: "specctl_charter_remove_spec", Description: "Remove one charter membership entry without deleting the tracking file."}, s.handleCharterRemoveSpec)
}

func (s *Server) handleContext(ctx context.Context, req *sdk.CallToolRequest, in contextInput) (*sdk.CallToolResult, any, error) {
	state, next, err := s.service.ReadContext(in.Target, in.File)
	if err != nil {
		return s.toolError(req, in, err)
	}
	responseState, focus := presenter.SplitStateFocus(state)
	envelope := presenter.ReadEnvelope(
		responseState,
		focus,
		presenter.DirectiveForReadMode(application.ReadSurfaceNextMode(state, next), next),
	)
	return s.toolResult(envelope), nil, nil
}

func (s *Server) handleDiff(ctx context.Context, req *sdk.CallToolRequest, in diffInput) (*sdk.CallToolResult, any, error) {
	state, next, err := s.service.ReadDiff(in.Target, in.Charter)
	if err != nil {
		return s.toolError(req, in, err)
	}
	responseState, focus := presenter.SplitStateFocus(state)
	mode := presenter.DirectiveForReadMode(application.ReadSurfaceNextMode(state, next), next)
	if in.Charter != "" {
		mode = presenter.None()
	}
	envelope := presenter.ReadEnvelope(responseState, focus, mode)
	return s.toolResult(envelope), nil, nil
}

func (s *Server) handleCharterCreate(ctx context.Context, req *sdk.CallToolRequest, in charterCreateInput) (*sdk.CallToolResult, any, error) {
	groups := make([]domain.CharterGroup, 0, len(in.Groups))
	for _, group := range in.Groups {
		groups = append(groups, domain.CharterGroup{Key: group.Key, Title: group.Title, Order: group.Order})
	}
	state, result, next, err := s.service.CreateCharter(application.CharterCreateRequest{
		Charter:     in.Charter,
		Title:       in.Title,
		Description: in.Description,
		Groups:      groups,
	})
	if err != nil {
		if charterExists, ok := err.(application.ErrCharterExists); ok {
			return s.toolResult(presenter.ErrorEnvelope(&presenter.Failure{
				Code:    "CHARTER_EXISTS",
				Message: err.Error(),
				State: map[string]any{
					"charter":       charterExists.Charter,
					"tracking_file": ".specs/" + charterExists.Charter + "/CHARTER.yaml",
				},
				Focus: map[string]any{},
				Next:  presenter.None(),
			})), nil, nil
		}
		return s.toolError(req, in, err)
	}
	responseState, focus := presenter.SplitStateFocus(state)
	envelope := presenter.WriteEnvelope(responseState, focus, result, presenter.Sequence(next))
	return s.toolResult(envelope), nil, nil
}

func (s *Server) handleSpecCreate(ctx context.Context, req *sdk.CallToolRequest, in specCreateInput) (*sdk.CallToolResult, any, error) {
	state, result, next, err := s.service.CreateSpec(application.SpecCreateRequest{
		Target:       in.Spec,
		Title:        in.Title,
		Doc:          in.Doc,
		Scope:        append([]string{}, in.Scope...),
		Group:        in.Group,
		GroupTitle:   in.GroupTitle,
		GroupOrder:   in.GroupOrder,
		Order:        in.Order,
		CharterNotes: in.CharterNotes,
		DependsOn:    append([]string{}, in.DependsOn...),
		Tags:         append([]string{}, in.Tags...),
	})
	if err != nil {
		return s.toolError(req, in, err)
	}
	responseState, focus := presenter.SplitStateFocus(state)
	envelope := presenter.WriteEnvelope(responseState, focus, result, presenter.Sequence(next))
	return s.toolResult(envelope), nil, nil
}

func (s *Server) handleDeltaAdd(ctx context.Context, req *sdk.CallToolRequest, in deltaAddInput) (*sdk.CallToolResult, any, error) {
	state, result, next, err := s.service.AddDelta(application.DeltaAddRequest{
		Target:              in.Spec,
		Intent:              domain.DeltaIntent(in.Intent),
		Area:                in.Area,
		Current:             in.Current,
		CurrentPresent:      in.Current != "",
		Targets:             in.Desired,
		TargetPresent:       in.Desired != "",
		Notes:               in.Notes,
		NotesPresent:        in.Notes != "",
		AffectsRequirements: append([]string{}, in.AffectsRequirements...),
	})
	if err != nil {
		return s.toolError(req, in, err)
	}
	responseState, focus := presenter.SplitStateFocus(state)
	envelope := presenter.WriteEnvelope(responseState, focus, result, presenter.Sequence(next))
	return s.toolResult(envelope), nil, nil
}

func (s *Server) handleDeltaStart(ctx context.Context, req *sdk.CallToolRequest, in deltaTransitionInput) (*sdk.CallToolResult, any, error) {
	return s.handleDeltaTransition(req, s.service.StartDelta, in)
}

func (s *Server) handleDeltaDefer(ctx context.Context, req *sdk.CallToolRequest, in deltaTransitionInput) (*sdk.CallToolResult, any, error) {
	return s.handleDeltaTransition(req, s.service.DeferDelta, in)
}

func (s *Server) handleDeltaResume(ctx context.Context, req *sdk.CallToolRequest, in deltaTransitionInput) (*sdk.CallToolResult, any, error) {
	return s.handleDeltaTransition(req, s.service.ResumeDelta, in)
}

func (s *Server) handleDeltaClose(ctx context.Context, req *sdk.CallToolRequest, in deltaTransitionInput) (*sdk.CallToolResult, any, error) {
	return s.handleDeltaTransition(req, s.service.CloseDelta, in)
}

func (s *Server) handleDeltaWithdraw(ctx context.Context, req *sdk.CallToolRequest, in deltaWithdrawInput) (*sdk.CallToolResult, any, error) {
	state, result, next, err := s.service.WithdrawDelta(application.DeltaWithdrawRequest{
		Target:  in.Spec,
		DeltaID: in.DeltaID,
		Reason:  in.Reason,
	})
	if err != nil {
		return s.toolError(req, in, err)
	}
	responseState, focus := presenter.SplitStateFocus(state)
	envelope := presenter.WriteEnvelope(responseState, focus, result, presenter.Sequence(next))
	return s.toolResult(envelope), nil, nil
}

func (s *Server) handleDeltaRebind(ctx context.Context, req *sdk.CallToolRequest, in deltaRebindInput) (*sdk.CallToolResult, any, error) {
	state, result, next, err := s.service.RebindDeltaRequirements(application.DeltaRebindRequest{
		Target:  in.Spec,
		DeltaID: in.DeltaID,
		From:    in.From,
		To:      in.To,
		Remove:  in.Remove,
		Reason:  in.Reason,
	})
	if err != nil {
		return s.toolError(req, in, err)
	}
	responseState, focus := presenter.SplitStateFocus(state)
	envelope := presenter.WriteEnvelope(responseState, focus, result, presenter.Sequence(next))
	return s.toolResult(envelope), nil, nil
}

func (s *Server) handleDeltaTransition(req *sdk.CallToolRequest, fn func(application.DeltaTransitionRequest) (application.SpecProjection, map[string]any, []any, error), in deltaTransitionInput) (*sdk.CallToolResult, any, error) {
	state, result, next, err := fn(application.DeltaTransitionRequest{
		Target:  in.Spec,
		DeltaID: in.DeltaID,
	})
	if err != nil {
		return s.toolError(req, in, err)
	}
	responseState, focus := presenter.SplitStateFocus(state)
	envelope := presenter.WriteEnvelope(responseState, focus, result, presenter.Sequence(next))
	return s.toolResult(envelope), nil, nil
}

func (s *Server) handleRequirementAdd(ctx context.Context, req *sdk.CallToolRequest, in requirementAddInput) (*sdk.CallToolResult, any, error) {
	state, result, next, err := s.service.AddRequirement(application.RequirementAddRequest{
		Target:  in.Spec,
		DeltaID: in.DeltaID,
		Gherkin: in.Gherkin,
	})
	if err != nil {
		return s.toolError(req, in, err)
	}
	responseState, focus := presenter.SplitStateFocus(state)
	envelope := presenter.WriteEnvelope(responseState, focus, result, presenter.Sequence(next))
	return s.toolResult(envelope), nil, nil
}

func (s *Server) handleRequirementReplace(ctx context.Context, req *sdk.CallToolRequest, in requirementReplaceInput) (*sdk.CallToolResult, any, error) {
	state, result, next, err := s.service.ReplaceRequirement(application.RequirementReplaceRequest{
		Target:        in.Spec,
		RequirementID: in.RequirementID,
		DeltaID:       in.DeltaID,
		Gherkin:       in.Gherkin,
	})
	if err != nil {
		return s.toolError(req, in, err)
	}
	responseState, focus := presenter.SplitStateFocus(state)
	envelope := presenter.WriteEnvelope(responseState, focus, result, presenter.Sequence(next))
	return s.toolResult(envelope), nil, nil
}

func (s *Server) handleRequirementRefresh(ctx context.Context, req *sdk.CallToolRequest, in requirementRefreshInput) (*sdk.CallToolResult, any, error) {
	state, result, next, err := s.service.RefreshRequirement(application.RequirementRefreshRequest{
		Target:        in.Spec,
		RequirementID: in.RequirementID,
		Gherkin:       in.Gherkin,
	})
	if err != nil {
		return s.toolError(req, in, err)
	}
	responseState, focus := presenter.SplitStateFocus(state)
	envelope := presenter.WriteEnvelope(responseState, focus, result, presenter.Sequence(next))
	return s.toolResult(envelope), nil, nil
}

func (s *Server) handleRequirementWithdraw(ctx context.Context, req *sdk.CallToolRequest, in requirementDeltaInput) (*sdk.CallToolResult, any, error) {
	state, result, next, err := s.service.WithdrawRequirement(application.RequirementDeltaRequest{
		Target:        in.Spec,
		RequirementID: in.RequirementID,
		DeltaID:       in.DeltaID,
	})
	if err != nil {
		return s.toolError(req, in, err)
	}
	responseState, focus := presenter.SplitStateFocus(state)
	envelope := presenter.WriteEnvelope(responseState, focus, result, presenter.Sequence(next))
	return s.toolResult(envelope), nil, nil
}

func (s *Server) handleRequirementStale(ctx context.Context, req *sdk.CallToolRequest, in requirementDeltaInput) (*sdk.CallToolResult, any, error) {
	state, result, next, err := s.service.StaleRequirement(application.RequirementDeltaRequest{
		Target:        in.Spec,
		RequirementID: in.RequirementID,
		DeltaID:       in.DeltaID,
	})
	if err != nil {
		return s.toolError(req, in, err)
	}
	responseState, focus := presenter.SplitStateFocus(state)
	envelope := presenter.WriteEnvelope(responseState, focus, result, presenter.Sequence(next))
	return s.toolResult(envelope), nil, nil
}

func (s *Server) handleRequirementVerify(ctx context.Context, req *sdk.CallToolRequest, in requirementVerifyInput) (*sdk.CallToolResult, any, error) {
	state, result, next, err := s.service.VerifyRequirement(application.RequirementVerifyRequest{
		Target:        in.Spec,
		RequirementID: in.RequirementID,
		TestFiles:     append([]string{}, in.TestFiles...),
	})
	if err != nil {
		return s.toolError(req, in, err)
	}
	responseState, focus := presenter.SplitStateFocus(state)
	envelope := presenter.WriteEnvelope(responseState, focus, result, presenter.Sequence(next))
	return s.toolResult(envelope), nil, nil
}

func (s *Server) handleRevisionBump(ctx context.Context, req *sdk.CallToolRequest, in revisionBumpInput) (*sdk.CallToolResult, any, error) {
	state, result, next, err := s.service.BumpRevision(application.RevisionBumpRequest{
		Target:     in.Spec,
		Checkpoint: in.Checkpoint,
		Summary:    in.Summary,
	})
	if err != nil {
		return s.toolError(req, in, err)
	}
	responseState, focus := presenter.SplitStateFocus(state)
	envelope := presenter.WriteEnvelope(responseState, focus, result, presenter.Sequence(next))
	return s.toolResult(envelope), nil, nil
}

func (s *Server) handleDocAdd(ctx context.Context, req *sdk.CallToolRequest, in docAddInput) (*sdk.CallToolResult, any, error) {
	state, result, next, err := s.service.DocAdd(application.DocAddRequest{
		Target: in.Spec,
		Doc:    in.Doc,
	})
	if err != nil {
		return s.toolError(req, in, err)
	}
	responseState, focus := presenter.SplitStateFocus(state)
	envelope := presenter.WriteEnvelope(responseState, focus, result, presenter.Sequence(next))
	return s.toolResult(envelope), nil, nil
}

func (s *Server) handleDocRemove(ctx context.Context, req *sdk.CallToolRequest, in docRemoveInput) (*sdk.CallToolResult, any, error) {
	state, result, next, err := s.service.DocRemove(application.DocRemoveRequest{
		Target: in.Spec,
		Doc:    in.Doc,
	})
	if err != nil {
		return s.toolError(req, in, err)
	}
	responseState, focus := presenter.SplitStateFocus(state)
	envelope := presenter.WriteEnvelope(responseState, focus, result, presenter.Sequence(next))
	return s.toolResult(envelope), nil, nil
}

func (s *Server) handleSync(ctx context.Context, req *sdk.CallToolRequest, in syncInput) (*sdk.CallToolResult, any, error) {
	state, result, next, err := s.service.Sync(application.SyncRequest{
		Target:     in.Spec,
		Checkpoint: in.Checkpoint,
		Summary:    in.Summary,
	})
	if err != nil {
		return s.toolError(req, in, err)
	}
	responseState, focus := presenter.SplitStateFocus(state)
	envelope := presenter.WriteEnvelope(responseState, focus, result, presenter.Sequence(next))
	return s.toolResult(envelope), nil, nil
}

func (s *Server) handleConfig(ctx context.Context, req *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, any, error) {
	state, err := s.service.ReadConfig()
	if err != nil {
		return s.toolError(req, in, err)
	}
	responseState, focus := presenter.SplitStateFocus(state)
	envelope := presenter.ReadEnvelope(responseState, focus, presenter.None())
	return s.toolResult(envelope), nil, nil
}

func (s *Server) handleConfigAddTag(ctx context.Context, req *sdk.CallToolRequest, in configTagInput) (*sdk.CallToolResult, any, error) {
	state, result, next, err := s.service.AddConfigTag(in.Tag)
	if err != nil {
		return s.toolError(req, in, err)
	}
	responseState, focus := presenter.SplitStateFocus(state)
	envelope := presenter.WriteEnvelope(responseState, focus, result, presenter.Sequence(next))
	return s.toolResult(envelope), nil, nil
}

func (s *Server) handleConfigRemoveTag(ctx context.Context, req *sdk.CallToolRequest, in configTagInput) (*sdk.CallToolResult, any, error) {
	state, result, next, err := s.service.RemoveConfigTag(in.Tag)
	if err != nil {
		return s.toolError(req, in, err)
	}
	responseState, focus := presenter.SplitStateFocus(state)
	envelope := presenter.WriteEnvelope(responseState, focus, result, presenter.Sequence(next))
	return s.toolResult(envelope), nil, nil
}

func (s *Server) handleConfigAddPrefix(ctx context.Context, req *sdk.CallToolRequest, in configPrefixInput) (*sdk.CallToolResult, any, error) {
	state, result, next, err := s.service.AddConfigPrefix(in.Prefix)
	if err != nil {
		return s.toolError(req, in, err)
	}
	responseState, focus := presenter.SplitStateFocus(state)
	envelope := presenter.WriteEnvelope(responseState, focus, result, presenter.Sequence(next))
	return s.toolResult(envelope), nil, nil
}

func (s *Server) handleConfigRemovePrefix(ctx context.Context, req *sdk.CallToolRequest, in configPrefixInput) (*sdk.CallToolResult, any, error) {
	state, result, next, err := s.service.RemoveConfigPrefix(in.Prefix)
	if err != nil {
		return s.toolError(req, in, err)
	}
	responseState, focus := presenter.SplitStateFocus(state)
	envelope := presenter.WriteEnvelope(responseState, focus, result, presenter.Sequence(next))
	return s.toolResult(envelope), nil, nil
}

func (s *Server) handleCharterAddSpec(ctx context.Context, req *sdk.CallToolRequest, in charterAddSpecInput) (*sdk.CallToolResult, any, error) {
	var groupTitle *string
	if trimmed := strings.TrimSpace(in.GroupTitle); trimmed != "" {
		groupTitle = &trimmed
	}
	state, result, next, err := s.service.AddSpecToCharter(application.CharterAddSpecRequest{
		Charter:    in.Charter,
		Slug:       in.Slug,
		Group:      in.Group,
		GroupTitle: groupTitle,
		GroupOrder: in.GroupOrder,
		Order:      in.Order,
		DependsOn:  append([]string{}, in.DependsOn...),
		Notes:      in.Notes,
	})
	if err != nil {
		return s.toolError(req, in, err)
	}
	responseState, focus := presenter.SplitStateFocus(state)
	envelope := presenter.WriteEnvelope(responseState, focus, result, presenter.Sequence(next))
	return s.toolResult(envelope), nil, nil
}

func (s *Server) handleCharterRemoveSpec(ctx context.Context, req *sdk.CallToolRequest, in charterRemoveSpecInput) (*sdk.CallToolResult, any, error) {
	state, result, next, err := s.service.RemoveSpecFromCharter(in.Charter, in.Slug)
	if err != nil {
		return s.toolError(req, in, err)
	}
	responseState, focus := presenter.SplitStateFocus(state)
	envelope := presenter.WriteEnvelope(responseState, focus, result, presenter.Sequence(next))
	return s.toolResult(envelope), nil, nil
}

func (s *Server) handleInit(ctx context.Context, req *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, any, error) {
	result, err := application.Init()
	if err != nil {
		return s.toolError(req, in, err)
	}
	state := result["state"]
	delete(result, "state")
	next := []any{
		map[string]any{
			"action":       "create_charter",
			"kind":         "run_command",
			"instructions": "Create your first charter to group related specs.",
			"template": map[string]any{
				"argv": []string{"specctl", "charter", "create", "<charter_name>"},
			},
		},
	}
	envelope := presenter.WriteEnvelope(state, nil, result, presenter.Sequence(next))
	return s.toolResult(envelope), nil, nil
}

func (s *Server) toolResult(envelope presenter.Envelope) *sdk.CallToolResult {
	envelope = adaptEnvelopeForMCP(envelope)
	data, encodeErr := presenter.MarshalEnvelope(envelope)
	if encodeErr != nil {
		fmt.Fprintf(panicLogger, "specctl mcp envelope encode failed: %v\n", encodeErr)
	}
	return &sdk.CallToolResult{
		Content: []sdk.Content{
			&sdk.TextContent{Text: string(data)},
		},
		IsError: envelope.Error != nil || encodeErr != nil,
	}
}

// toolError converts an error from the application layer into the standard
// MCP tool result envelope. *Failure errors keep their tipified codes;
// anything else gets the report_issue hint via UnexpectedErrorEnvelope.
func (s *Server) toolError(req *sdk.CallToolRequest, in any, err error) (*sdk.CallToolResult, any, error) {
	envelope := presenter.ClassifyError(err, presenter.UnexpectedContext{
		Tool:  toolNameFromRequest(req),
		Input: in,
	})
	return s.toolResult(envelope), nil, nil
}

func toolNameFromRequest(req *sdk.CallToolRequest) string {
	if req == nil || req.Params == nil {
		return ""
	}
	return req.Params.Name
}

// maxStackBytes caps the captured stack trace so the report_issue body
// stays under GitHub's 65536-char issue body limit even after redaction.
const maxStackBytes = 8 * 1024

// wrapHandler decorates a typed tool handler with a panic recover that
// converts the panic into a structured envelope (with the report_issue hint)
// instead of crashing the stdio process. The recovered panic is logged to
// stderr with the full stack trace. Re-panics inside the recovery path are
// swallowed (logged) so the stdio session never dies.
func wrapHandler[In any](
	toolName string,
	s *Server,
	h func(context.Context, *sdk.CallToolRequest, In) (*sdk.CallToolResult, any, error),
) func(context.Context, *sdk.CallToolRequest, In) (*sdk.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdk.CallToolRequest, in In) (result *sdk.CallToolResult, meta any, err error) {
		// completed flips to true only if h returns normally. Any path
		// where the deferred function fires without completed=true means
		// a panic happened — including panic(nil) on older toolchains.
		completed := false
		defer func() {
			r := recover()
			if completed && r == nil {
				return
			}
			panicVal := r
			if panicVal == nil {
				panicVal = "panic(nil)"
			}
			defer func() {
				// Belt-and-suspenders: a re-panic inside the recovery
				// envelope construction must not escape this goroutine.
				if rr := recover(); rr != nil {
					fmt.Fprintf(panicLogger, "specctl mcp double-panic in %s: %v\n", toolName, rr)
					result = nil
					meta = nil
					err = fmt.Errorf("specctl internal error: %v", rr)
				}
			}()
			stack := truncateStack(string(debug.Stack()), maxStackBytes)
			fmt.Fprintf(panicLogger, "specctl mcp panic in %s: %v\n%s\n", toolName, panicVal, stack)
			envelope := presenter.UnexpectedErrorEnvelope(
				fmt.Errorf("panic: %v", panicVal),
				presenter.UnexpectedContext{
					Tool:       toolName,
					Input:      in,
					PanicValue: panicVal,
					Stack:      stack,
				},
			)
			result = s.toolResult(envelope)
			meta = nil
			err = nil
		}()
		result, meta, err = h(ctx, req, in)
		completed = true
		return
	}
}

func truncateStack(stack string, max int) string {
	if max <= 0 || len(stack) <= max {
		return stack
	}
	return stack[:max] + fmt.Sprintf("\n…(stack truncated, %d more bytes)\n", len(stack)-max)
}

// addTool registers a typed handler with the SDK after wrapping it with
// the panic-recover middleware.
func addTool[In any](
	s *Server,
	tool *sdk.Tool,
	h func(context.Context, *sdk.CallToolRequest, In) (*sdk.CallToolResult, any, error),
) {
	sdk.AddTool(s.server, tool, wrapHandler(tool.Name, s, h))
}

func adaptEnvelopeForMCP(envelope presenter.Envelope) presenter.Envelope {
	envelope.Next = adaptDirectiveForMCP(envelope.Next)
	return envelope
}

func adaptDirectiveForMCP(next presenter.Directive) presenter.Directive {
	next = presenter.CoalesceDirective(next)
	if next.Mode == "none" {
		return next
	}
	next.Steps = adaptNextActions(next.Steps, true)
	next.Options = adaptNextActions(next.Options, false)
	if len(next.Steps) == 0 && len(next.Options) == 0 {
		return presenter.None()
	}
	return next
}

func adaptNextActions(actions []any, stopOnUnsupported bool) []any {
	if len(actions) == 0 {
		return nil
	}
	adapted := make([]any, 0, len(actions))
	for _, raw := range actions {
		action, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		mcpHint, ok := mcpHintForAction(action)
		if !ok {
			blocker := cloneMap(action)
			blocker["mcp"] = map[string]any{
				"available": false,
				"reason":    "unsupported_in_v1",
			}
			adapted = append(adapted, blocker)
			if stopOnUnsupported {
				break
			}
			continue
		}
		cloned := cloneMap(action)
		cloned["mcp"] = mcpHint
		adapted = append(adapted, cloned)
	}
	return adapted
}

func mcpHintForAction(action map[string]any) (map[string]any, bool) {
	name, _ := action["action"].(string)
	tool := ""
	input := map[string]any{}

	switch name {
	case "create_charter":
		tool = "specctl_charter_create"
	case "create_spec":
		tool = "specctl_spec_create"
	case "delta_add_add":
		tool = "specctl_delta_add"
		input["intent"] = "add"
	case "delta_add_change":
		tool = "specctl_delta_add"
		input["intent"] = "change"
	case "delta_add_remove":
		tool = "specctl_delta_add"
		input["intent"] = "remove"
	case "delta_add_repair":
		tool = "specctl_delta_add"
		input["intent"] = "repair"
	case "add_requirement":
		tool = "specctl_requirement_add"
	case "refresh_requirement":
		tool = "specctl_requirement_refresh"
	case "verify_requirement":
		tool = "specctl_requirement_verify"
	case "start_delta":
		tool = "specctl_delta_start"
	case "close_delta":
		tool = "specctl_delta_close"
	case "rev_bump":
		tool = "specctl_revision_bump"
	case "sync", "sync_checkpoint":
		tool = "specctl_sync"
	case "doc_add":
		tool = "specctl_doc_add"
	case "doc_remove":
		tool = "specctl_doc_remove"
	case "report_issue":
		// External link step (URL in template); the agent surfaces it
		// to the user instead of calling an MCP tool.
		return map[string]any{"available": true, "kind": "external_link"}, true
	default:
		return nil, false
	}

	if template, ok := action["template"].(map[string]any); ok {
		applyLegacyArgvHint(input, tool, template["argv"])
	}
	return map[string]any{
		"tool":  tool,
		"input": input,
	}, true
}

func applyLegacyArgvHint(input map[string]any, tool string, argv any) {
	args := stringSlice(argv)
	if len(args) == 0 {
		return
	}
	switch tool {
	case "specctl_charter_create":
		if len(args) >= 4 {
			input["charter"] = args[3]
		}
	case "specctl_spec_create":
		if len(args) >= 4 {
			input["spec"] = args[3]
		}
	case "specctl_delta_add", "specctl_delta_start", "specctl_delta_close", "specctl_requirement_add", "specctl_requirement_verify", "specctl_requirement_refresh":
		if len(args) >= 4 {
			input["spec"] = args[3]
		}
	case "specctl_requirement_replace", "specctl_requirement_withdraw", "specctl_requirement_stale":
		if len(args) >= 5 {
			input["spec"] = args[3]
			input["requirement_id"] = args[4]
		}
	case "specctl_delta_defer", "specctl_delta_resume":
		if len(args) >= 5 {
			input["spec"] = args[3]
			input["delta_id"] = args[4]
		}
	case "specctl_revision_bump":
		if len(args) >= 4 {
			input["spec"] = args[3]
		}
	case "specctl_sync":
		if len(args) >= 3 {
			input["spec"] = args[2]
		}
	}

	switch tool {
	case "specctl_delta_start", "specctl_delta_defer", "specctl_delta_resume", "specctl_delta_close":
		if len(args) >= 5 {
			input["delta_id"] = args[4]
		}
	case "specctl_requirement_add":
		for i := 0; i < len(args)-1; i++ {
			if args[i] == "--delta" {
				input["delta_id"] = args[i+1]
			}
		}
	case "specctl_requirement_verify", "specctl_requirement_refresh":
		if len(args) >= 5 {
			input["requirement_id"] = args[4]
		}
	case "specctl_revision_bump", "specctl_sync":
		for i := 0; i < len(args)-1; i++ {
			if args[i] == "--checkpoint" {
				input["checkpoint"] = args[i+1]
			}
		}
	}

	if tool == "specctl_requirement_verify" {
		testFiles := make([]string, 0, 1)
		for i := 0; i < len(args)-1; i++ {
			if args[i] == "--test-file" {
				testFiles = append(testFiles, args[i+1])
			}
		}
		if len(testFiles) > 0 {
			input["test_files"] = testFiles
		}
	}
}

func stringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string{}, typed...)
	case []any:
		out := make([]string, 0, len(typed))
		for _, raw := range typed {
			text, ok := raw.(string)
			if ok {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func cloneMap(src map[string]any) map[string]any {
	cloned := make(map[string]any, len(src))
	for key, value := range src {
		cloned[key] = cloneAny(value)
	}
	return cloned
}

func cloneAny(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMap(typed)
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, cloneAny(item))
		}
		return out
	default:
		return typed
	}
}
