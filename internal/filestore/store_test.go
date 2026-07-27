package filestore

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

func TestStageFinalizeAndOpen(t *testing.T) {
	store, err := New(t.TempDir(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	payload := append([]byte("%PDF-1.7\n"), bytes.Repeat([]byte("x"), 32)...)
	staged, err := store.Stage(context.Background(), bytes.NewReader(payload), "report.pdf", "application/pdf")
	if err != nil {
		t.Fatal(err)
	}
	if staged.MediaKind != "document" || staged.SizeBytes != int64(len(payload)) || len(staged.SHA256) != 64 {
		t.Fatalf("staged=%#v", staged)
	}
	if err := store.Finalize(staged.StorageKey); err != nil {
		t.Fatal(err)
	}
	file, err := store.Open(staged.StorageKey)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	actual, _ := io.ReadAll(file)
	if !bytes.Equal(actual, payload) {
		t.Fatal("stored bytes changed")
	}
}

func TestCleanupOrphansPreservesKnownPendingKeys(t *testing.T) {
	store, err := New(t.TempDir(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := store.Stage(context.Background(), bytes.NewReader([]byte("plain")), "one.txt", "text/plain")
	second, _ := store.Stage(context.Background(), bytes.NewReader([]byte("plain")), "two.txt", "text/plain")
	removed, err := store.CleanupOrphans(map[string]bool{first.StorageKey: true}, time.Now().Add(time.Minute))
	if err != nil || removed != 1 {
		t.Fatalf("removed=%d error=%v", removed, err)
	}
	if err := store.Finalize(first.StorageKey); err != nil {
		t.Fatal(err)
	}
	if err := store.Finalize(second.StorageKey); err == nil {
		t.Fatal("orphan staging file still existed")
	}
}

func TestStageRejectsOversizeAndExecutable(t *testing.T) {
	store, err := New(t.TempDir(), 8)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Stage(context.Background(), bytes.NewReader([]byte("plain text longer")), "note.txt", "text/plain")
	if !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("oversize error=%v", err)
	}
	_, err = store.Stage(context.Background(), bytes.NewReader([]byte("MZ executable")), "bad.exe", "application/octet-stream")
	if !errors.Is(err, ErrTypeNotAllowed) {
		t.Fatalf("type error=%v", err)
	}
}
