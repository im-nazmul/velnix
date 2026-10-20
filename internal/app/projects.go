package app

import (
	"context"
	"time"

	"velnix/internal/domain"
	"velnix/internal/state"
)

type ProjectStore interface {
	Status() (state.Status, error)
	PlanProject(state.ProjectChange) (state.ProjectPlan, error)
	ApplyProject(context.Context, state.ProjectChange, string) (state.Status, error)
}

type ProjectRequest struct {
	state.ProjectChange
	DryRun         bool
	ApprovedPlanID string
}

type ProjectService struct{ Store ProjectStore }

func (s ProjectService) Run(ctx context.Context, request ProjectRequest) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if request.DryRun && request.ApprovedPlanID != "" {
		return nil, domain.E(domain.Invalid, "--dry-run cannot be combined with --approve")
	}
	switch request.Action {
	case "list", "inspect":
		if request.DryRun || request.ApprovedPlanID != "" {
			return nil, domain.E(domain.Invalid, "project queries are read-only")
		}
		st, err := s.Store.Status()
		if err != nil {
			return nil, err
		}
		if st.RecoveryRequired {
			return nil, domain.E(domain.Conflict, "interrupted operation requires state recover")
		}
		if request.Action == "list" {
			projects := st.Registry.Projects
			if projects == nil {
				projects = []domain.Project{}
			}
			return projects, nil
		}
		if !domain.ValidProjectName(request.Name) {
			return nil, domain.E(domain.Invalid, "invalid project name")
		}
		for _, p := range st.Registry.Projects {
			if p.Name == request.Name {
				return p, nil
			}
		}
		return nil, domain.E(domain.Unavailable, "project is not linked")
	case "link", "unlink":
		if request.ApprovedPlanID == "" {
			return s.Store.PlanProject(request.ProjectChange)
		}
		applyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return s.Store.ApplyProject(applyCtx, request.ProjectChange, request.ApprovedPlanID)
	default:
		return nil, domain.E(domain.Invalid, "unknown project subcommand %q", request.Action)
	}
}
