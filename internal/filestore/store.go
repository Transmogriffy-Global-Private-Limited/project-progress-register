// Package filestore owns private attachment bytes and filesystem recovery primitives.
package filestore

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
)

var storageKeyPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

var (
	ErrFileTooLarge   = errors.New("attachment exceeds the configured size limit")
	ErrFileEmpty      = errors.New("attachment is empty")
	ErrTypeNotAllowed = errors.New("attachment type is not allowed")
)

type StagedFile struct {
	StorageKey   string
	OriginalName string
	ReportedMIME string
	DetectedMIME string
	MediaKind    string
	SizeBytes    int64
	SHA256       string
}

type ReadSeekCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}

type Store struct {
	root     string
	staging  string
	data     string
	maxBytes int64
}

func New(root string, maxBytes int64) (*Store, error) {
	root = strings.TrimSpace(root)
	if root == "" || maxBytes < 1 {
		return nil, errors.New("attachment root and positive size limit are required")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve attachment root: %w", err)
	}
	store := &Store{root: absolute, staging: filepath.Join(absolute, ".staging"), data: filepath.Join(absolute, "data"), maxBytes: maxBytes}
	for _, directory := range []string{store.root, store.staging, store.data} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return nil, fmt.Errorf("create attachment directory: %w", err)
		}
	}
	return store, nil
}

func (s *Store) Root() string { return s.root }

func (s *Store) Stage(ctx context.Context, source io.Reader, originalName, reportedMIME string) (StagedFile, error) {
	if source == nil {
		return StagedFile{}, ErrFileEmpty
	}
	name, err := safeDisplayName(originalName)
	if err != nil {
		return StagedFile{}, err
	}
	reader := bufio.NewReaderSize(source, 512)
	header, peekErr := reader.Peek(512)
	if peekErr != nil && !errors.Is(peekErr, io.EOF) && !errors.Is(peekErr, bufio.ErrBufferFull) {
		return StagedFile{}, fmt.Errorf("inspect attachment: %w", peekErr)
	}
	if len(header) == 0 {
		return StagedFile{}, ErrFileEmpty
	}
	detectedMIME, mediaKind, err := classify(name, header)
	if err != nil {
		return StagedFile{}, err
	}
	key, err := randomKey()
	if err != nil {
		return StagedFile{}, err
	}
	temporaryPath := s.stagingPath(key)
	file, err := os.OpenFile(temporaryPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return StagedFile{}, fmt.Errorf("create staged attachment: %w", err)
	}
	remove := true
	defer func() {
		_ = file.Close()
		if remove {
			_ = os.Remove(temporaryPath)
		}
	}()
	hash := sha256.New()
	limited := &io.LimitedReader{R: reader, N: s.maxBytes + 1}
	written, err := io.Copy(io.MultiWriter(file, hash), &contextReader{ctx: ctx, reader: limited})
	if err != nil {
		return StagedFile{}, fmt.Errorf("stream attachment: %w", err)
	}
	if written > s.maxBytes {
		return StagedFile{}, ErrFileTooLarge
	}
	if written == 0 {
		return StagedFile{}, ErrFileEmpty
	}
	if err := file.Sync(); err != nil {
		return StagedFile{}, fmt.Errorf("sync staged attachment: %w", err)
	}
	if err := file.Close(); err != nil {
		return StagedFile{}, fmt.Errorf("close staged attachment: %w", err)
	}
	remove = false
	return StagedFile{
		StorageKey: key, OriginalName: name, ReportedMIME: strings.TrimSpace(reportedMIME),
		DetectedMIME: detectedMIME, MediaKind: mediaKind, SizeBytes: written, SHA256: hex.EncodeToString(hash.Sum(nil)),
	}, nil
}

func (s *Store) Finalize(storageKey string) error {
	if !storageKeyPattern.MatchString(storageKey) {
		return errors.New("invalid attachment storage key")
	}
	finalPath := s.finalPath(storageKey)
	if _, err := os.Stat(finalPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect final attachment: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o700); err != nil {
		return fmt.Errorf("create attachment shard: %w", err)
	}
	if err := os.Rename(s.stagingPath(storageKey), finalPath); err != nil {
		return fmt.Errorf("finalize attachment: %w", err)
	}
	return nil
}

func (s *Store) Open(storageKey string) (ReadSeekCloser, error) {
	if !storageKeyPattern.MatchString(storageKey) {
		return nil, errors.New("invalid attachment storage key")
	}
	file, err := os.Open(s.finalPath(storageKey))
	if err != nil {
		return nil, fmt.Errorf("open attachment: %w", err)
	}
	return file, nil
}

func (s *Store) DiscardStaged(storageKey string) error {
	if !storageKeyPattern.MatchString(storageKey) {
		return errors.New("invalid attachment storage key")
	}
	err := os.Remove(s.stagingPath(storageKey))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *Store) ExistsFinal(storageKey string) bool {
	if !storageKeyPattern.MatchString(storageKey) {
		return false
	}
	_, err := os.Stat(s.finalPath(storageKey))
	return err == nil
}

func (s *Store) CleanupOrphans(keep map[string]bool, olderThan time.Time) (int, error) {
	entries, err := os.ReadDir(s.staging)
	if err != nil {
		return 0, fmt.Errorf("list staged attachments: %w", err)
	}
	removed := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".part") {
			continue
		}
		key := strings.TrimSuffix(entry.Name(), ".part")
		if !storageKeyPattern.MatchString(key) || keep[key] {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return removed, fmt.Errorf("inspect staged attachment: %w", err)
		}
		if !info.ModTime().Before(olderThan) {
			continue
		}
		if err := os.Remove(filepath.Join(s.staging, entry.Name())); err != nil && !errors.Is(err, os.ErrNotExist) {
			return removed, fmt.Errorf("remove orphan staged attachment: %w", err)
		}
		removed++
	}
	return removed, nil
}

func (s *Store) stagingPath(key string) string { return filepath.Join(s.staging, key+".part") }
func (s *Store) finalPath(key string) string   { return filepath.Join(s.data, key[:2], key) }

func randomKey() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate attachment storage key: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

func safeDisplayName(value string) (string, error) {
	value = filepath.Base(strings.TrimSpace(value))
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
	if value == "" || value == "." || len([]byte(value)) > 255 {
		return "", errors.New("attachment filename must contain 1-255 UTF-8 bytes")
	}
	return value, nil
}

func classify(name string, header []byte) (string, string, error) {
	detected := strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(header), ";")[0]))
	extension := strings.ToLower(filepath.Ext(name))
	allowed := map[string]string{
		"image/jpeg": "image", "image/png": "image", "image/gif": "image", "image/webp": "image",
		"application/pdf": "document",
		"video/mp4":       "video", "video/webm": "video", "video/quicktime": "video", "video/x-msvideo": "video",
	}
	if detected == "text/plain" {
		switch extension {
		case ".txt", ".md":
			return "text/plain", "document", nil
		case ".csv":
			return "text/csv", "document", nil
		default:
			return "", "", ErrTypeNotAllowed
		}
	}
	if kind, ok := allowed[detected]; ok {
		return detected, kind, nil
	}
	zipDocuments := map[string]string{
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		".odt":  "application/vnd.oasis.opendocument.text", ".ods": "application/vnd.oasis.opendocument.spreadsheet", ".odp": "application/vnd.oasis.opendocument.presentation",
	}
	if detected == "application/zip" {
		if mimeType, ok := zipDocuments[extension]; ok {
			return mimeType, "document", nil
		}
	}
	if len(header) >= 8 && string(header[:8]) == "\xd0\xcf\x11\xe0\xa1\xb1\x1a\xe1" {
		legacy := map[string]string{".doc": "application/msword", ".xls": "application/vnd.ms-excel", ".ppt": "application/vnd.ms-powerpoint"}
		if mimeType, ok := legacy[extension]; ok {
			return mimeType, "document", nil
		}
	}
	return "", "", ErrTypeNotAllowed
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(buffer []byte) (int, error) {
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
		return r.reader.Read(buffer)
	}
}
