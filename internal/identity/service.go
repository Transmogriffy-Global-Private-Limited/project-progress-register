package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	loginWindow        = 15 * time.Minute
	loginBlock         = 15 * time.Minute
	maxLoginFailures   = 5
	maxLoginIdentifier = 254
)

// Repository owns atomic PostgreSQL transitions used by identity orchestration.
type Repository interface {
	UserCount(context.Context) (int, error)
	BootstrapAdmin(context.Context, NewUser, AuditEvent) (User, error)
	FindUserByIdentifier(context.Context, string) (UserRecord, error)
	IsLoginBlocked(context.Context, []byte, string, time.Time) (bool, error)
	RecordLoginFailure(context.Context, []byte, string, time.Time, time.Duration, int, time.Duration, AuditEvent) error
	CreateLoginSession(context.Context, string, []byte, time.Time, []byte, string, AuditEvent) error
	LookupSession(context.Context, []byte, time.Time) (Session, error)
	RevokeSession(context.Context, []byte, AuditEvent) error
	AppendAudit(context.Context, AuditEvent) error
}

// UserRecord includes the password verifier needed only inside authentication.
type UserRecord struct {
	User
	PasswordHash string
}

// Service implements identity policy independent of HTTP and PostgreSQL details.
type Service struct {
	repository          Repository
	passwords           *passwordHasher
	dummyPasswordHash   string
	csrfKey             []byte
	sessionTTL          time.Duration
	bootstrapConfigured bool
	bootstrapTokenHash  string
	now                 func() time.Time
}

// ServiceConfig contains security-sensitive identity configuration.
type ServiceConfig struct {
	CSRFKey        []byte
	SessionTTL     time.Duration
	BootstrapToken string
}

// NewService constructs identity policy and its fixed dummy password verifier.
func NewService(ctx context.Context, repository Repository, cfg ServiceConfig) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("identity repository is required")
	}
	if len(cfg.CSRFKey) != 32 {
		return nil, fmt.Errorf("identity CSRF key must contain exactly 32 bytes")
	}
	if cfg.SessionTTL < 15*time.Minute || cfg.SessionTTL > 7*24*time.Hour {
		return nil, fmt.Errorf("identity session TTL is outside the supported range")
	}
	hasher := newPasswordHasher()
	dummyHash, err := hasher.Hash(ctx, "dummy-password-verifier-not-a-real-credential")
	if err != nil {
		return nil, fmt.Errorf("create dummy password verifier: %w", err)
	}
	service := &Service{
		repository:          repository,
		passwords:           hasher,
		dummyPasswordHash:   dummyHash,
		csrfKey:             append([]byte(nil), cfg.CSRFKey...),
		sessionTTL:          cfg.SessionTTL,
		bootstrapConfigured: cfg.BootstrapToken != "",
		bootstrapTokenHash:  string(hashToken(cfg.BootstrapToken)),
		now:                 time.Now,
	}
	return service, nil
}

// BootstrapAvailable reports whether the guarded first-user workflow can run.
func (s *Service) BootstrapAvailable(ctx context.Context) (bool, error) {
	if !s.bootstrapConfigured {
		return false, nil
	}
	count, err := s.repository.UserCount(ctx)
	if err != nil {
		return false, fmt.Errorf("count users: %w", err)
	}
	return count == 0, nil
}

// BootstrapAdmin creates the initial Admin after all trust-boundary checks.
func (s *Service) BootstrapAdmin(ctx context.Context, input BootstrapInput, audit AuditContext) (User, error) {
	if !s.bootstrapConfigured {
		return User{}, ErrBootstrapUnavailable
	}
	if !secureEqual(string(hashToken(input.BootstrapToken)), s.bootstrapTokenHash) {
		if err := s.repository.AppendAudit(ctx, AuditEvent{Action: "identity.bootstrap_failed", TargetType: "user", Outcome: "denied", Context: cleanAuditContext(audit), Details: map[string]any{"reason": "invalid_bootstrap_token"}}); err != nil {
			return User{}, fmt.Errorf("audit denied bootstrap: %w", err)
		}
		return User{}, ErrBootstrapDenied
	}
	username, err := validateUsername(input.Username)
	if err != nil {
		return User{}, err
	}
	email, err := validateEmail(input.Email)
	if err != nil {
		return User{}, err
	}
	if err := validatePassword(input.Password); err != nil {
		return User{}, err
	}
	passwordHash, err := s.passwords.Hash(ctx, input.Password)
	if err != nil {
		return User{}, fmt.Errorf("hash bootstrap password: %w", err)
	}
	user, err := s.repository.BootstrapAdmin(ctx, NewUser{Username: username, Email: email, PasswordHash: passwordHash, Role: RoleAdmin}, AuditEvent{
		Action: "identity.bootstrap_succeeded", TargetType: "user", Outcome: "succeeded", Context: cleanAuditContext(audit), Details: map[string]any{"role": RoleAdmin},
	})
	if err != nil {
		if errors.Is(err, ErrBootstrapUnavailable) {
			return User{}, ErrBootstrapUnavailable
		}
		return User{}, fmt.Errorf("bootstrap Admin: %w", err)
	}
	return user, nil
}

// Login authenticates without disclosing whether an account exists or is disabled.
func (s *Service) Login(ctx context.Context, input LoginInput, audit AuditContext) (LoginResult, error) {
	audit = cleanAuditContext(audit)
	identifier := normalizeIdentifier(input.Identifier)
	identifierDigest := identifierHash(identifier)
	if identifier == "" || len(identifier) > maxLoginIdentifier || len([]byte(input.Password)) > 128 {
		if err := s.recordFailure(ctx, identifierDigest, audit, "invalid_credentials"); err != nil {
			return LoginResult{}, err
		}
		return LoginResult{}, ErrInvalidCredentials
	}
	blocked, err := s.repository.IsLoginBlocked(ctx, identifierDigest, audit.ClientIP, s.now())
	if err != nil {
		return LoginResult{}, fmt.Errorf("check login throttle: %w", err)
	}
	if blocked {
		if err := s.repository.AppendAudit(ctx, AuditEvent{Action: "auth.login_throttled", TargetType: "user", Outcome: "denied", Context: audit, Details: map[string]any{"identifier_hash": fmt.Sprintf("%x", identifierDigest)}}); err != nil {
			return LoginResult{}, fmt.Errorf("audit throttled login: %w", err)
		}
		return LoginResult{}, ErrLoginThrottled
	}

	record, findErr := s.repository.FindUserByIdentifier(ctx, identifier)
	hashToCheck := s.dummyPasswordHash
	if findErr == nil {
		hashToCheck = record.PasswordHash
	} else if !errors.Is(findErr, ErrNotFound) {
		return LoginResult{}, fmt.Errorf("find login account: %w", findErr)
	}
	matched, err := s.passwords.Verify(ctx, hashToCheck, input.Password)
	if err != nil {
		return LoginResult{}, fmt.Errorf("verify password: %w", err)
	}
	if findErr != nil || !matched || !record.Enabled {
		if err := s.recordFailure(ctx, identifierDigest, audit, "invalid_credentials"); err != nil {
			return LoginResult{}, err
		}
		return LoginResult{}, ErrInvalidCredentials
	}

	token, tokenHash, err := newSessionToken()
	if err != nil {
		return LoginResult{}, err
	}
	expiresAt := s.now().Add(s.sessionTTL)
	if err := s.repository.CreateLoginSession(ctx, record.ID, tokenHash, expiresAt, identifierDigest, audit.ClientIP, AuditEvent{
		ActorUserID: record.ID, Action: "auth.login_succeeded", TargetType: "user", TargetID: record.ID, Outcome: "succeeded", Context: audit,
	}); err != nil {
		return LoginResult{}, fmt.Errorf("create login session: %w", err)
	}
	return LoginResult{User: record.User, SessionToken: token, CSRFToken: deriveCSRFToken(s.csrfKey, token), ExpiresAt: expiresAt}, nil
}

// CurrentSession authenticates a raw opaque token against current database state.
func (s *Service) CurrentSession(ctx context.Context, token string) (Session, error) {
	if strings.TrimSpace(token) == "" {
		return Session{}, ErrUnauthenticated
	}
	session, err := s.repository.LookupSession(ctx, hashToken(token), s.now())
	if err != nil {
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrUnauthenticated) {
			return Session{}, ErrUnauthenticated
		}
		return Session{}, fmt.Errorf("lookup session: %w", err)
	}
	return session, nil
}

// CSRFToken returns the deterministic session-bound write token.
func (s *Service) CSRFToken(sessionToken string) string {
	return deriveCSRFToken(s.csrfKey, sessionToken)
}

// ValidateCSRF performs a constant-time comparison against the session-bound token.
func (s *Service) ValidateCSRF(sessionToken, supplied string) error {
	if supplied == "" || !secureEqual(deriveCSRFToken(s.csrfKey, sessionToken), supplied) {
		return ErrCSRFInvalid
	}
	return nil
}

// Logout revokes the presented session and appends its audit event atomically.
func (s *Service) Logout(ctx context.Context, sessionToken string, user User, audit AuditContext) error {
	if sessionToken == "" {
		return nil
	}
	if err := s.repository.RevokeSession(ctx, hashToken(sessionToken), AuditEvent{
		ActorUserID: user.ID, Action: "auth.logout_succeeded", TargetType: "user", TargetID: user.ID, Outcome: "succeeded", Context: cleanAuditContext(audit),
	}); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

func (s *Service) recordFailure(ctx context.Context, digest []byte, audit AuditContext, reason string) error {
	if err := s.repository.RecordLoginFailure(ctx, digest, audit.ClientIP, s.now(), loginWindow, maxLoginFailures, loginBlock, AuditEvent{
		Action: "auth.login_failed", TargetType: "user", Outcome: "failed", Context: audit, Details: map[string]any{"reason": reason, "identifier_hash": fmt.Sprintf("%x", digest)},
	}); err != nil {
		return fmt.Errorf("record login failure: %w", err)
	}
	return nil
}

func cleanAuditContext(value AuditContext) AuditContext {
	value.RequestID = truncate(strings.TrimSpace(value.RequestID), 64)
	value.ClientIP = strings.TrimSpace(value.ClientIP)
	value.UserAgent = truncate(strings.TrimSpace(value.UserAgent), 512)
	return value
}

func truncate(value string, maximum int) string {
	if len(value) <= maximum {
		return value
	}
	value = value[:maximum]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}
