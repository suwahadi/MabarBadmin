package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/suwahadi/mabarbadmin/internal/database"
	"github.com/suwahadi/mabarbadmin/internal/database/sqlc"
	"github.com/suwahadi/mabarbadmin/internal/media"
	"github.com/suwahadi/mabarbadmin/internal/settings"
)

// UserError carries a message that is safe to show to the user (in Indonesian).
type UserError struct{ Msg string }

func (e *UserError) Error() string { return e.Msg }

func userErr(msg string) error { return &UserError{Msg: msg} }

func IsUserError(err error) (string, bool) {
	var ue *UserError
	if errors.As(err, &ue) {
		return ue.Msg, true
	}
	return "", false
}

var ErrNotFound = errors.New("not found")

type Actor struct {
	Type string
	ID   *int64
}

func AdminActor(id int64) Actor  { return Actor{Type: "admin", ID: &id} }
func MemberActor(id int64) Actor { return Actor{Type: "member", ID: &id} }
func GuestActor() Actor          { return Actor{Type: "guest"} }
func SystemActor() Actor         { return Actor{Type: "system"} }

type Service struct {
	Pool     *pgxpool.Pool
	Q        *sqlc.Queries
	Settings *settings.Store
	Media    *media.Service
	Log      *slog.Logger
	Now      func() time.Time

	rndMu sync.Mutex
	rnd   *rand.Rand
}

func New(pool *pgxpool.Pool, st *settings.Store, m *media.Service, log *slog.Logger) *Service {
	return &Service{
		Pool:     pool,
		Q:        sqlc.New(pool),
		Settings: st,
		Media:    m,
		Log:      log,
		Now:      time.Now,
		rnd:      rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), rand.Uint64())),
	}
}

func (s *Service) withRand(f func(r *rand.Rand)) {
	s.rndMu.Lock()
	defer s.rndMu.Unlock()
	f(s.rnd)
}

func (s *Service) tx(ctx context.Context, fn func(q *sqlc.Queries) error) error {
	return database.InTx(ctx, s.Pool, func(q *sqlc.Queries, _ pgx.Tx) error { return fn(q) })
}

func logActivity(ctx context.Context, q *sqlc.Queries, a Actor, entity string, id int64, action string, data map[string]any) error {
	raw := []byte("{}")
	if len(data) > 0 {
		raw, _ = json.Marshal(data)
	}
	return q.CreateActivityLog(ctx, sqlc.CreateActivityLogParams{
		ActorType: a.Type, ActorID: a.ID, EntityType: entity, EntityID: id, Action: action, Data: raw,
	})
}

func notFound(err error) error {
	if database.IsNotFound(err) {
		return ErrNotFound
	}
	return err
}
