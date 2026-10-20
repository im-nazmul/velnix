package app

import (
	"context"
	"time"

	"velnix/internal/domain"
	"velnix/internal/state"
)

type StateStore interface {
	Status() (state.Status, error)
	Plan(string) (state.Plan, error)
	Apply(context.Context, string, string) (state.Status, error)
}

type StateRequest struct {
	Action         string
	DryRun         bool
	ApprovedPlanID string
}

type StateResult struct {
	Plan   *state.Plan
	Status *state.Status
}

// StateService shares preview/approval policy across present and future transports.
type StateService struct{ Store StateStore }

func (s StateService) Run(ctx context.Context, request StateRequest) (StateResult, error) {
	if request.DryRun && request.ApprovedPlanID != "" {
		return StateResult{}, domain.E(domain.Invalid, "--dry-run cannot be combined with --approve")
	}
	if request.Action == "status" {
		if request.DryRun || request.ApprovedPlanID != "" {
			return StateResult{}, domain.E(domain.Invalid, "state status is read-only")
		}
		status, err := s.Store.Status()
		return StateResult{Status: &status}, err
	}
	if request.Action != "init" && request.Action != "recover" {
		return StateResult{}, domain.E(domain.Invalid, "unknown state subcommand %q", request.Action)
	}
	if request.ApprovedPlanID == "" {
		plan, err := s.Store.Plan(request.Action)
		return StateResult{Plan: &plan}, err
	}
	applyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	status, err := s.Store.Apply(applyCtx, request.Action, request.ApprovedPlanID)
	return StateResult{Status: &status}, err
}
