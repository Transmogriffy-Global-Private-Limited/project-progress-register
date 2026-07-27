package identity

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestPasswordHashAndValidation(t *testing.T) {
	t.Parallel()
	hasher := newPasswordHasher()
	first, err := hasher.Hash(context.Background(), "twelve-chars-plus")
	if err != nil {
		t.Fatal(err)
	}
	second, err := hasher.Hash(context.Background(), "twelve-chars-plus")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("password hashes reused a salt")
	}
	matched, err := hasher.Verify(context.Background(), first, "twelve-chars-plus")
	if err != nil || !matched {
		t.Fatalf("Verify() = %v, %v", matched, err)
	}
	matched, err = hasher.Verify(context.Background(), first, "different-password")
	if err != nil || matched {
		t.Fatalf("Verify(wrong) = %v, %v", matched, err)
	}
	if validatePassword("too-short") == nil {
		t.Fatal("short password was accepted")
	}
}

func TestSessionTokenAndCSRF(t *testing.T) {
	t.Parallel()
	token, digest, err := newSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if token == "" || len(digest) != 32 {
		t.Fatalf("token=%q digest bytes=%d", token, len(digest))
	}
	key := make([]byte, 32)
	csrf := deriveCSRFToken(key, token)
	if csrf == "" || !secureEqual(csrf, deriveCSRFToken(key, token)) || secureEqual(csrf, deriveCSRFToken(key, token+"x")) {
		t.Fatal("CSRF derivation is not session-bound")
	}
}

func TestAuditDefaultsDetailsToObject(t *testing.T) {
	t.Parallel()
	execer := &captureExecer{}
	err := insertAudit(context.Background(), execer, AuditEvent{Action: "auth.logout_succeeded", TargetType: "user", Outcome: "succeeded", Context: AuditContext{RequestID: "request-12345678", ClientIP: "127.0.0.1"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(execer.arguments[8].([]byte)); got != "{}" {
		t.Fatalf("audit details = %q", got)
	}
}

type captureExecer struct{ arguments []any }

func (c *captureExecer) Exec(_ context.Context, _ string, arguments ...any) (pgconn.CommandTag, error) {
	c.arguments = arguments
	return pgconn.CommandTag{}, nil
}

func TestServiceLoginOutcomes(t *testing.T) {
	repository := &fakeRepository{}
	service, err := NewService(context.Background(), repository, ServiceConfig{CSRFKey: make([]byte, 32), SessionTTL: time.Hour, BootstrapToken: "bootstrap-token-at-least-24-chars"})
	if err != nil {
		t.Fatal(err)
	}
	hash, err := service.passwords.Hash(context.Background(), "correct-password-value")
	if err != nil {
		t.Fatal(err)
	}
	repository.record = UserRecord{User: User{ID: "00000000-0000-0000-0000-000000000001", Username: "admin", Email: "admin@example.com", Role: RoleAdmin, Enabled: true}, PasswordHash: hash}
	audit := AuditContext{RequestID: "request-12345678", ClientIP: "127.0.0.1", UserAgent: "test"}

	result, err := service.Login(context.Background(), LoginInput{Identifier: "ADMIN", Password: "correct-password-value"}, audit)
	if err != nil || result.SessionToken == "" || result.CSRFToken == "" {
		t.Fatalf("Login() = %#v, %v", result, err)
	}
	if repository.sessionCreates != 1 || repository.lastEvent.Action != "auth.login_succeeded" {
		t.Fatalf("session transition = %d %#v", repository.sessionCreates, repository.lastEvent)
	}

	_, err = service.Login(context.Background(), LoginInput{Identifier: "admin", Password: "wrong-password"}, audit)
	if !errors.Is(err, ErrInvalidCredentials) || repository.failures != 1 || repository.lastEvent.Action != "auth.login_failed" {
		t.Fatalf("wrong password = %v failures=%d event=%s", err, repository.failures, repository.lastEvent.Action)
	}

	repository.notFound = true
	_, err = service.Login(context.Background(), LoginInput{Identifier: "missing", Password: "wrong-password"}, audit)
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("unknown user error = %v", err)
	}

	repository.blocked = true
	_, err = service.Login(context.Background(), LoginInput{Identifier: "admin", Password: "anything"}, audit)
	if !errors.Is(err, ErrLoginThrottled) || repository.lastEvent.Action != "auth.login_throttled" {
		t.Fatalf("blocked error = %v event=%s", err, repository.lastEvent.Action)
	}
}

func TestServiceBootstrapAndSession(t *testing.T) {
	repository := &fakeRepository{}
	service, err := NewService(context.Background(), repository, ServiceConfig{CSRFKey: make([]byte, 32), SessionTTL: time.Hour, BootstrapToken: "bootstrap-token-at-least-24-chars"})
	if err != nil {
		t.Fatal(err)
	}
	audit := AuditContext{RequestID: "request-12345678", ClientIP: "127.0.0.1"}
	_, err = service.BootstrapAdmin(context.Background(), BootstrapInput{BootstrapToken: "wrong", Username: "admin", Email: "admin@example.com", Password: "correct-password-value"}, audit)
	if !errors.Is(err, ErrBootstrapDenied) || repository.lastEvent.Action != "identity.bootstrap_failed" {
		t.Fatalf("denied bootstrap = %v event=%s", err, repository.lastEvent.Action)
	}
	user, err := service.BootstrapAdmin(context.Background(), BootstrapInput{BootstrapToken: "bootstrap-token-at-least-24-chars", Username: "Admin", Email: "ADMIN@example.com", Password: "correct-password-value"}, audit)
	if err != nil || user.Username != "admin" || repository.lastEvent.Action != "identity.bootstrap_succeeded" {
		t.Fatalf("bootstrap = %#v, %v event=%s", user, err, repository.lastEvent.Action)
	}

	repository.session = Session{ID: "session", User: user, ExpiresAt: time.Now().Add(time.Hour)}
	session, err := service.CurrentSession(context.Background(), "raw-token")
	if err != nil || session.ID != "session" {
		t.Fatalf("CurrentSession() = %#v, %v", session, err)
	}
	csrf := service.CSRFToken("raw-token")
	if service.ValidateCSRF("raw-token", csrf) != nil || !errors.Is(service.ValidateCSRF("raw-token", "bad"), ErrCSRFInvalid) {
		t.Fatal("CSRF validation mismatch")
	}
	if err := service.Logout(context.Background(), "raw-token", user, audit); err != nil || repository.lastEvent.Action != "auth.logout_succeeded" {
		t.Fatalf("Logout() = %v event=%s", err, repository.lastEvent.Action)
	}
}

func TestAccountAdministrationService(t *testing.T) {
	repository := &fakeRepository{}
	service, err := NewService(context.Background(), repository, ServiceConfig{CSRFKey: make([]byte, 32), SessionTTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	admin := User{ID: "00000000-0000-0000-0000-000000000001", Username: "admin", Role: RoleAdmin, Enabled: true}
	audit := AuditContext{RequestID: "request-12345678", ClientIP: "127.0.0.1"}

	created, err := service.CreateUser(context.Background(), admin, CreateUserInput{Username: "member.one", Email: "member@example.com", Role: RoleMember}, audit)
	if err != nil || created.TemporaryPassword == "" || !created.User.MustChangePassword {
		t.Fatalf("CreateUser() = %#v, %v", created, err)
	}
	if repository.lastEvent.Action != "identity.user_created" {
		t.Fatalf("create audit = %s", repository.lastEvent.Action)
	}

	enabled := false
	updated, err := service.UpdateUser(context.Background(), admin, created.User.ID, UpdateUserInput{Role: RoleMember, Enabled: &enabled, ExpectedVersion: 1}, audit)
	if err != nil || updated.Enabled || updated.Version != 2 {
		t.Fatalf("UpdateUser() = %#v, %v", updated, err)
	}

	reset, err := service.ResetPassword(context.Background(), admin, created.User.ID, audit)
	if err != nil || reset.TemporaryPassword == "" || repository.lastEvent.Action != "identity.password_reset" {
		t.Fatalf("ResetPassword() = %#v, %v", reset, err)
	}

	if err := service.ChangePassword(context.Background(), created.User, ChangePasswordInput{Password: "replacement-password-value"}, audit); err != nil {
		t.Fatalf("ChangePassword() = %v", err)
	}
	if repository.lastEvent.Action != "identity.password_changed" {
		t.Fatalf("change audit = %s", repository.lastEvent.Action)
	}
}

func TestAccountAdministrationRequiresReadyAdmin(t *testing.T) {
	repository := &fakeRepository{}
	service, err := NewService(context.Background(), repository, ServiceConfig{CSRFKey: make([]byte, 32), SessionTTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	audit := AuditContext{RequestID: "request-12345678", ClientIP: "127.0.0.1"}
	_, err = service.ListUsers(context.Background(), User{ID: "00000000-0000-0000-0000-000000000002", Role: RoleMember, Enabled: true}, audit)
	if !errors.Is(err, ErrForbidden) || repository.lastEvent.Action != "authorization.admin_users_denied" {
		t.Fatalf("member ListUsers() = %v event=%s", err, repository.lastEvent.Action)
	}
	_, err = service.ListUsers(context.Background(), User{ID: "00000000-0000-0000-0000-000000000001", Role: RoleAdmin, Enabled: true, MustChangePassword: true}, audit)
	if !errors.Is(err, ErrPasswordChangeNeeded) {
		t.Fatalf("temporary Admin ListUsers() = %v", err)
	}
}

type fakeRepository struct {
	count                    int
	record                   UserRecord
	notFound, blocked        bool
	failures, sessionCreates int
	session                  Session
	lastEvent                AuditEvent
	users                    []User
	credentialUser           User
}

func (f *fakeRepository) UserCount(context.Context) (int, error) { return f.count, nil }
func (f *fakeRepository) BootstrapAdmin(_ context.Context, input NewUser, event AuditEvent) (User, error) {
	f.lastEvent = event
	return User{ID: "00000000-0000-0000-0000-000000000001", Username: input.Username, Email: input.Email, Role: input.Role, Enabled: true}, nil
}
func (f *fakeRepository) FindUserByIdentifier(context.Context, string) (UserRecord, error) {
	if f.notFound {
		return UserRecord{}, ErrNotFound
	}
	return f.record, nil
}
func (f *fakeRepository) IsLoginBlocked(context.Context, []byte, string, time.Time) (bool, error) {
	return f.blocked, nil
}
func (f *fakeRepository) RecordLoginFailure(_ context.Context, _ []byte, _ string, _ time.Time, _ time.Duration, _ int, _ time.Duration, event AuditEvent) error {
	f.failures++
	f.lastEvent = event
	return nil
}
func (f *fakeRepository) CreateLoginSession(_ context.Context, _ string, _ []byte, _ time.Time, _ []byte, _ string, event AuditEvent) error {
	f.sessionCreates++
	f.lastEvent = event
	return nil
}
func (f *fakeRepository) LookupSession(context.Context, []byte, time.Time) (Session, error) {
	if f.session.ID == "" {
		return Session{}, ErrUnauthenticated
	}
	return f.session, nil
}
func (f *fakeRepository) RevokeSession(_ context.Context, _ []byte, event AuditEvent) error {
	f.lastEvent = event
	return nil
}
func (f *fakeRepository) AppendAudit(_ context.Context, event AuditEvent) error {
	f.lastEvent = event
	return nil
}
func (f *fakeRepository) ListUsers(context.Context) ([]User, error) { return f.users, nil }
func (f *fakeRepository) CreateUser(_ context.Context, input NewUser, event AuditEvent) (User, error) {
	f.lastEvent = event
	user := f.credentialUser
	if user.ID == "" {
		user = User{ID: "00000000-0000-0000-0000-000000000002", Username: input.Username, Email: input.Email, Role: input.Role, Enabled: true, MustChangePassword: true, Version: 1}
	}
	return user, nil
}
func (f *fakeRepository) UpdateUser(_ context.Context, _ string, input UpdateUserInput, event AuditEvent) (User, error) {
	f.lastEvent = event
	return User{ID: "00000000-0000-0000-0000-000000000002", Role: input.Role, Enabled: *input.Enabled, Version: input.ExpectedVersion + 1}, nil
}
func (f *fakeRepository) ResetPassword(_ context.Context, targetID, _ string, event AuditEvent) (User, error) {
	f.lastEvent = event
	return User{ID: targetID, Username: "member", Enabled: true, MustChangePassword: true, Version: 2}, nil
}
func (f *fakeRepository) ChangePassword(_ context.Context, _ string, _ string, event AuditEvent) error {
	f.lastEvent = event
	return nil
}
func (f *fakeRepository) ListIdentityAudit(context.Context, int) ([]AuditRecord, error) {
	return []AuditRecord{{Action: "identity.user_created"}}, nil
}
