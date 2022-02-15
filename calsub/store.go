package calsub

import (
	"context"

	"github.com/target/goalert/auth/authtoken"
	"github.com/target/goalert/config"
	"github.com/target/goalert/keyring"
	"github.com/target/goalert/oncall"
	"github.com/target/goalert/permission"
	"github.com/target/goalert/util/sqlutil"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Store allows the lookup and management of calendar subscriptions
type Store struct {
	keys keyring.Keyring
	oc   oncall.Store
}

// NewStore will create a new Store with the given parameters.
func NewStore(ctx context.Context, apiKeyring keyring.Keyring, oc oncall.Store) (*Store, error) {
	return &Store{
		keys: apiKeyring,
		oc:   oc,
	}, nil
}

// Authorize will return an authorized context associated with the given token. If the token is invalid
// or otherwise can not be authenticated, an error is returned.
func Authorize(ctx context.Context, tok authtoken.Token) (context.Context, error) {
	if tok.Type != authtoken.TypeCalSub {
		return ctx, permission.NewAccessDenied("invalid token")
	}

	cfg := config.FromContext(ctx)
	if cfg.General.DisableCalendarSubscriptions {
		return nil, permission.NewAccessDenied("disabled by administrator")
	}

	sCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	sCtx = permission.SystemContext(sCtx, "CalSubAuthorize")

	var cs Subscription
	db := sqlutil.FromContext(ctx).
		WithContext(sCtx).
		Model(&cs).
		Where("not disabled").
		Where("id = ?", tok.ID).
		Where("date_trunc('second', created_at) = ?", tok.CreatedAt).
		Clauses(clause.Returning{}).
		UpdateColumn("last_access", gorm.Expr("now()"))
	if db.Error != nil {
		return ctx, db.Error
	}
	if db.RowsAffected == 0 {
		return ctx, permission.NewAccessDenied("invalid token")
	}

	return permission.UserSourceContext(ctx, cs.UserID, permission.RoleUser, &permission.SourceInfo{
		Type: permission.SourceTypeCalendarSubscription,
		ID:   tok.ID.String(),
	}), nil
}

// SignToken will sign a token for the given subscription.
func (s *Store) SignToken(ctx context.Context, cs *Subscription) (string, error) {
	if cs.token == nil {
		return "", nil
	}

	return cs.token.Encode(s.keys.Sign)
}
