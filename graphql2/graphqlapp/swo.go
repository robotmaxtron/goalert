package graphqlapp

import (
	"context"

	"github.com/target/goalert/graphql2"
	"github.com/target/goalert/validation"
)

func (m *Mutation) SwoAction(ctx context.Context, action graphql2.SWOAction) (bool, error) {
	if m.SWO == nil {
		return false, validation.NewGenericError("not in SWO mode")
	}

	var err error
	switch action {
	case graphql2.SWOActionPing:
		err = m.SWO.SendPing(ctx)
	case graphql2.SWOActionReset:
		err = m.SWO.SendReset(ctx)
	case graphql2.SWOActionExecute:
		err = m.SWO.SendExecute(ctx)
	default:
		return false, validation.NewGenericError("invalid SWO action")
	}

	return err == nil, err
}

func (a *Query) SwoStatus(ctx context.Context) (*graphql2.SWOStatus, error) {
	if a.SWO == nil {
		return nil, validation.NewGenericError("not in SWO mode")
	}

	s := a.SWO.Status()
	var nodes []graphql2.SWONode
	for _, n := range s.Nodes {
		nodes = append(nodes, graphql2.SWONode{
			ID:       n.ID.String(),
			OldValid: n.OldValid,
			NewValid: n.NewValid,
			CanExec:  n.CanExec,
			Status:   n.Status,
		})
	}

	return &graphql2.SWOStatus{
		IsIdle:  s.IsIdle,
		IsDone:  s.IsDone,
		Details: s.Details,
		Nodes:   nodes,
	}, nil
}