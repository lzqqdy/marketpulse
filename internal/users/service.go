package users

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"mime/multipart"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/lzqqdy/marketpulse/internal/config"
	"github.com/lzqqdy/marketpulse/internal/platform/cache"
	platformredis "github.com/lzqqdy/marketpulse/internal/platform/redis"
	"github.com/lzqqdy/marketpulse/internal/platform/upload"
	usersmigrate "github.com/lzqqdy/marketpulse/internal/users/migrate"
)

var phoneRe = regexp.MustCompile(`^1\d{10}$`)

// Service is the users module facade.
type Service interface {
	Enabled() bool
	Login(ctx context.Context, phone, password, clientIP string) (LoginResult, error)
	Logout(ctx context.Context, token string) error
	Me(ctx context.Context, token string) (User, error)
	UpdateProfile(ctx context.Context, token string, in UpdateProfileInput) (User, error)
	ChangePassword(ctx context.Context, token, oldPassword, newPassword string) error
	UploadAvatar(ctx context.Context, token string, fh *multipart.FileHeader) (User, error)
	UserIDFromToken(ctx context.Context, token string) (int64, error)
}

type service struct {
	cfg      config.UsersConfig
	repo     *repository
	sessions *sessionStore
	uploads  *upload.Store
	guard    *loginGuard
}

// BootstrapArgs bundles deps for the users module.
type BootstrapArgs struct {
	Users  config.UsersConfig
	DB     *sql.DB
	Redis  *platformredis.Client
	Upload *upload.Store
}

// Bootstrap opens migrations and returns a Service. Requires MySQL + Redis.
func Bootstrap(ctx context.Context, args BootstrapArgs) (Service, error) {
	cfg := args.Users
	if !cfg.Enabled {
		return &service{cfg: cfg}, nil
	}
	if args.DB == nil {
		return nil, fmt.Errorf("users: mysql required when users.enabled")
	}
	if args.Redis == nil {
		return nil, fmt.Errorf("users: redis required when users.enabled")
	}
	if cfg.IsAutoMigrate() {
		if err := usersmigrate.Run(ctx, args.DB); err != nil {
			return nil, err
		}
		slog.Info("users migrations applied")
	}
	svc := &service{
		cfg:      cfg,
		repo:     newRepository(args.DB, cache.New(args.Redis, "mp:cache:users:")),
		sessions: newSessionStore(args.Redis, cfg.SessionTTL),
		uploads:  args.Upload,
		guard:    newLoginGuard(args.Redis, cfg.Security),
	}
	if err := svc.ensureSeed(ctx); err != nil {
		return nil, err
	}
	return svc, nil
}

func (s *service) Enabled() bool {
	return s != nil && s.cfg.Enabled && s.repo != nil && s.sessions != nil
}

func (s *service) ensureSeed(ctx context.Context) error {
	phone := strings.TrimSpace(s.cfg.Seed.Phone)
	pass := s.cfg.Seed.Password
	if phone == "" || pass == "" {
		return nil
	}
	if _, err := s.repo.GetByPhone(ctx, phone); err == nil {
		return nil
	} else if err != ErrNotFound {
		return err
	}
	hash, err := hashPassword(pass)
	if err != nil {
		return err
	}
	name := strings.TrimSpace(s.cfg.Seed.DisplayName)
	if name == "" {
		name = "管理员"
	}
	if _, err := s.repo.Create(ctx, phone, hash, name); err != nil {
		return fmt.Errorf("users seed: %w", err)
	}
	slog.Info("users seed account created", "phone", phone)
	return nil
}

func (s *service) Login(ctx context.Context, phone, password, clientIP string) (LoginResult, error) {
	if !s.Enabled() {
		return LoginResult{}, ErrDisabled
	}
	phone = normalizePhone(phone)
	clientIP = strings.TrimSpace(clientIP)
	if !phoneRe.MatchString(phone) || password == "" {
		padLoginTiming(password)
		return LoginResult{}, ErrInvalidCredentials
	}
	if err := s.guard.allowAttempt(ctx, clientIP, phone); err != nil {
		return LoginResult{}, err
	}

	row, err := s.repo.GetByPhone(ctx, phone)
	if err != nil {
		if err == ErrNotFound {
			padLoginTiming(password)
			s.guard.recordFailure(ctx, phone)
			return LoginResult{}, ErrInvalidCredentials
		}
		return LoginResult{}, err
	}
	if !checkPassword(row.PasswordHash, password) {
		s.guard.recordFailure(ctx, phone)
		return LoginResult{}, ErrInvalidCredentials
	}
	s.guard.clearFailures(ctx, phone)
	token, exp, err := s.sessions.Create(ctx, row.ID)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Token: token, ExpiresAt: exp, User: row.toPublic()}, nil
}

func (s *service) Logout(ctx context.Context, token string) error {
	if !s.Enabled() {
		return ErrDisabled
	}
	return s.sessions.Delete(ctx, token)
}

func (s *service) Me(ctx context.Context, token string) (User, error) {
	id, err := s.UserIDFromToken(ctx, token)
	if err != nil {
		return User{}, err
	}
	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return User{}, err
	}
	return row.toPublic(), nil
}

func (s *service) UpdateProfile(ctx context.Context, token string, in UpdateProfileInput) (User, error) {
	id, err := s.UserIDFromToken(ctx, token)
	if err != nil {
		return User{}, err
	}
	if err := validateProfile(in); err != nil {
		return User{}, err
	}
	row, err := s.repo.UpdateProfile(ctx, id, in)
	if err != nil {
		return User{}, err
	}
	return row.toPublic(), nil
}

func (s *service) UploadAvatar(ctx context.Context, token string, fh *multipart.FileHeader) (User, error) {
	id, err := s.UserIDFromToken(ctx, token)
	if err != nil {
		return User{}, err
	}
	if s.uploads == nil {
		return User{}, fmt.Errorf("%w: upload unavailable", ErrInvalidInput)
	}
	url, err := s.uploads.SaveAvatar(id, fh)
	if err != nil {
		return User{}, fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
	}
	row, err := s.repo.UpdateProfile(ctx, id, UpdateProfileInput{AvatarURL: &url})
	if err != nil {
		return User{}, err
	}
	return row.toPublic(), nil
}

func (s *service) ChangePassword(ctx context.Context, token, oldPassword, newPassword string) error {
	id, err := s.UserIDFromToken(ctx, token)
	if err != nil {
		return err
	}
	if utf8.RuneCountInString(newPassword) < 6 {
		return fmt.Errorf("%w: password too short", ErrInvalidInput)
	}
	row, err := s.repo.loadFull(ctx, id)
	if err != nil {
		return err
	}
	if !checkPassword(row.PasswordHash, oldPassword) {
		return ErrWrongPassword
	}
	hash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	return s.repo.UpdatePassword(ctx, id, hash)
}

func (s *service) UserIDFromToken(ctx context.Context, token string) (int64, error) {
	if !s.Enabled() {
		return 0, ErrDisabled
	}
	return s.sessions.UserID(ctx, token)
}

func normalizePhone(phone string) string {
	phone = strings.TrimSpace(phone)
	phone = strings.ReplaceAll(phone, " ", "")
	return phone
}

func validateProfile(in UpdateProfileInput) error {
	if in.DisplayName != nil {
		n := utf8.RuneCountInString(strings.TrimSpace(*in.DisplayName))
		if n > 64 {
			return fmt.Errorf("%w: displayName too long", ErrInvalidInput)
		}
	}
	if in.AvatarURL != nil && utf8.RuneCountInString(*in.AvatarURL) > 512 {
		return fmt.Errorf("%w: avatarUrl too long", ErrInvalidInput)
	}
	if in.Email != nil {
		e := strings.TrimSpace(*in.Email)
		if e != "" && (!strings.Contains(e, "@") || utf8.RuneCountInString(e) > 128) {
			return fmt.Errorf("%w: invalid email", ErrInvalidInput)
		}
	}
	if in.WechatPushToken != nil && utf8.RuneCountInString(*in.WechatPushToken) > 256 {
		return fmt.Errorf("%w: wechatPushToken too long", ErrInvalidInput)
	}
	return nil
}
