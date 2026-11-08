package service

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/suwahadi/mabarbadmin/internal/auth"
	"github.com/suwahadi/mabarbadmin/internal/database"
	"github.com/suwahadi/mabarbadmin/internal/database/sqlc"
	"github.com/suwahadi/mabarbadmin/internal/domain"
	"github.com/suwahadi/mabarbadmin/internal/locale"
)

const SessionTTL = 30 * 24 * time.Hour

func validatePassword(p string) error {
	if len(p) < 8 {
		return userErr("Password minimal 8 karakter.")
	}
	if len(p) > 128 {
		return userErr("Password terlalu panjang.")
	}
	return nil
}

type SignupInput struct {
	Name     string
	Phone    string
	Password string
	Level    string
	Role     string
}

// CreateUser registers a member (or admin from the CLI) and attaches earlier guest registrations
// that used the same WhatsApp number, so their history counts for the leaderboard.
func (s *Service) CreateUser(ctx context.Context, in SignupInput) (sqlc.User, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len(in.Name) > 60 {
		return sqlc.User{}, userErr("Nama wajib diisi (maks 60 karakter).")
	}
	phone, err := locale.NormalizePhone(in.Phone)
	if err != nil {
		return sqlc.User{}, userErr("Nomor WA tidak valid. Contoh: 0812-3456-7890.")
	}
	if in.Level == "" {
		in.Level = domain.LevelBeginner
	}
	if !domain.ValidLevel(in.Level) {
		return sqlc.User{}, userErr("Pilih level.")
	}
	if in.Role == "" {
		in.Role = "member"
	}
	if err := validatePassword(in.Password); err != nil {
		return sqlc.User{}, err
	}
	if _, err := s.Q.GetUserByPhone(ctx, phone); err == nil {
		return sqlc.User{}, userErr("Nomor WA ini sudah terdaftar. Silakan masuk.")
	} else if !database.IsNotFound(err) {
		return sqlc.User{}, err
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return sqlc.User{}, err
	}
	var u sqlc.User
	err = s.tx(ctx, func(q *sqlc.Queries) error {
		u, err = q.CreateUser(ctx, sqlc.CreateUserParams{Phone: phone, Name: in.Name, PasswordHash: hash, Role: in.Role, Level: in.Level})
		if err != nil {
			return err
		}
		if err := q.LinkGuestRegistrations(ctx, sqlc.LinkGuestRegistrationsParams{UserID: &u.ID, Phone: phone}); err != nil {
			return err
		}
		return logActivity(ctx, q, MemberActor(u.ID), "user", u.ID, "signed_up", map[string]any{"role": in.Role})
	})
	return u, err
}

// Authenticate verifies credentials and returns a new session token for the cookie.
func (s *Service) Authenticate(ctx context.Context, rawPhone, password, userAgent string) (sqlc.User, string, error) {
	invalid := userErr("Nomor WA atau password salah.")
	phone, err := locale.NormalizePhone(rawPhone)
	if err != nil {
		return sqlc.User{}, "", invalid
	}
	u, err := s.Q.GetUserByPhone(ctx, phone)
	if err != nil {
		if database.IsNotFound(err) {
			_, _ = auth.VerifyPassword(password, dummyHash())
			return sqlc.User{}, "", invalid
		}
		return sqlc.User{}, "", err
	}
	ok, err := auth.VerifyPassword(password, u.PasswordHash)
	if err != nil || !ok || !u.IsActive {
		return sqlc.User{}, "", invalid
	}
	token, err := s.StartSession(ctx, u.ID, userAgent)
	return u, token, err
}

// dummyHash keeps login timing similar for unknown numbers.
var dummyHash = sync.OnceValue(func() string {
	h, _ := auth.HashPassword("not-a-real-password")
	return h
})

func (s *Service) StartSession(ctx context.Context, userID int64, userAgent string) (string, error) {
	token, hash, err := auth.NewToken()
	if err != nil {
		return "", err
	}
	if len(userAgent) > 255 {
		userAgent = userAgent[:255]
	}
	err = s.Q.CreateAuthSession(ctx, sqlc.CreateAuthSessionParams{TokenHash: hash, UserID: userID, ExpiresAt: s.Now().Add(SessionTTL), UserAgent: userAgent})
	return token, err
}

func (s *Service) ChangePassword(ctx context.Context, userID int64, current, next string) error {
	u, err := s.Q.GetUser(ctx, userID)
	if err != nil {
		return notFound(err)
	}
	if ok, _ := auth.VerifyPassword(current, u.PasswordHash); !ok {
		return userErr("Password saat ini salah.")
	}
	if err := validatePassword(next); err != nil {
		return err
	}
	hash, err := auth.HashPassword(next)
	if err != nil {
		return err
	}
	return s.Q.UpdateUserPassword(ctx, sqlc.UpdateUserPasswordParams{ID: userID, PasswordHash: hash, MustChangePassword: false})
}

// ResetPassword gives the member a temporary password that must be changed after login.
func (s *Service) ResetPassword(ctx context.Context, a Actor, userID int64) (string, error) {
	temp, err := auth.TempPassword()
	if err != nil {
		return "", err
	}
	hash, err := auth.HashPassword(temp)
	if err != nil {
		return "", err
	}
	err = s.tx(ctx, func(q *sqlc.Queries) error {
		if err := q.UpdateUserPassword(ctx, sqlc.UpdateUserPasswordParams{ID: userID, PasswordHash: hash, MustChangePassword: true}); err != nil {
			return err
		}
		if err := q.DeleteUserAuthSessions(ctx, userID); err != nil {
			return err
		}
		return logActivity(ctx, q, a, "user", userID, "password_reset", nil)
	})
	return temp, err
}

type ProfileInput struct {
	Name, Level                                string
	RefundBank, RefundAccount, RefundAccountNm string
}

func (s *Service) UpdateProfile(ctx context.Context, userID int64, in ProfileInput) error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len(in.Name) > 60 {
		return userErr("Nama wajib diisi (maks 60 karakter).")
	}
	if !domain.ValidLevel(in.Level) {
		return userErr("Pilih level.")
	}
	_, err := s.Q.UpdateUserProfile(ctx, sqlc.UpdateUserProfileParams{
		ID: userID, Name: in.Name, Level: in.Level, RefundBankName: strings.TrimSpace(in.RefundBank),
		RefundAccountNumber: strings.TrimSpace(in.RefundAccount), RefundAccountName: strings.TrimSpace(in.RefundAccountNm),
	})
	return err
}

func (s *Service) AdminUpdateUser(ctx context.Context, a Actor, userID int64, name, level string, active bool) error {
	name = strings.TrimSpace(name)
	if name == "" || !domain.ValidLevel(level) {
		return userErr("Nama dan level wajib diisi.")
	}
	return s.tx(ctx, func(q *sqlc.Queries) error {
		if err := q.UpdateUserAdmin(ctx, sqlc.UpdateUserAdminParams{ID: userID, Name: name, Level: level, IsActive: active}); err != nil {
			return err
		}
		if !active {
			if err := q.DeleteUserAuthSessions(ctx, userID); err != nil {
				return err
			}
		}
		return logActivity(ctx, q, a, "user", userID, "updated_by_admin", map[string]any{"level": level, "active": active})
	})
}
