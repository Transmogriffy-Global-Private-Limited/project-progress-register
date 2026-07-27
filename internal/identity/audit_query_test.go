package identity

import (
	"context"
	"testing"
	"time"
)

func TestAuditCursorRoundTripAndValidation(t *testing.T) {
	want := auditCursor{OccurredAt: time.Date(2026, 7, 27, 12, 0, 0, 123, time.UTC), ID: "33333333-3333-4333-8333-333333333333"}
	encoded, err := encodeAuditCursor(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeAuditCursor(encoded)
	if err != nil || !got.OccurredAt.Equal(want.OccurredAt) || got.ID != want.ID {
		t.Fatalf("cursor round trip = %+v, %v", got, err)
	}
	if _, err := validateAuditQuery(AuditQuery{Limit: 201}); err == nil {
		t.Fatal("expected excessive limit to fail")
	}
	if _, err := validateAuditQuery(AuditQuery{Cursor: "invalid"}); err == nil {
		t.Fatal("expected invalid cursor to fail")
	}
}

func TestFullAuditRequiresAdmin(t *testing.T) {
	service, err := NewService(context.Background(), &fakeRepository{}, ServiceConfig{CSRFKey: make([]byte, 32), SessionTTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.ListAudit(context.Background(), User{ID: "member", Role: RoleMember, Enabled: true}, AuditQuery{}, AuditContext{RequestID: "request-12345678", ClientIP: "127.0.0.1"})
	if err != ErrForbidden {
		t.Fatalf("member audit error = %v", err)
	}
}
