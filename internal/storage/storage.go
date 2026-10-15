// Package storage provides object storage for job logs and project artifacts.
package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ─── Types ────────────────────────────────────────────────────────────────────

// ObjectInfo contains metadata about a stored object.
type ObjectInfo struct {
	Key          string
	Size         int64
	ContentType  string
	LastModified time.Time
}

// ─── Store Interface ──────────────────────────────────────────────────────────

// Store is the interface for object storage operations.
type Store interface {
	// Put stores an object.
	Put(ctx context.Context, key string, data io.Reader, size int64, contentType string) error
	// Get retrieves an object.
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	// Delete removes an object.
	Delete(ctx context.Context, key string) error
	// Exists checks if an object exists.
	Exists(ctx context.Context, key string) (bool, error)
	// List lists objects with a prefix.
	List(ctx context.Context, prefix string) ([]ObjectInfo, error)
	// URL returns a presigned URL for direct access.
	URL(ctx context.Context, key string, expiry time.Duration) (string, error)
}

// ─── Local Store ──────────────────────────────────────────────────────────────

// LocalStore implements Store using the local filesystem.
type LocalStore struct {
	basePath string
}

// NewLocalStore creates a new local filesystem store.
func NewLocalStore(basePath string) (*LocalStore, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("create base path: %w", err)
	}
	return &LocalStore{basePath: basePath}, nil
}

// Put stores an object on the local filesystem.
func (s *LocalStore) Put(ctx context.Context, key string, data io.Reader, size int64, contentType string) error {
	path := filepath.Join(s.basePath, key)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, data); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// Get retrieves an object from the local filesystem.
func (s *LocalStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	path := filepath.Join(s.basePath, key)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	return f, nil
}

// Delete removes an object from the local filesystem.
func (s *LocalStore) Delete(ctx context.Context, key string) error {
	path := filepath.Join(s.basePath, key)
	return os.Remove(path)
}

// Exists checks if an object exists.
func (s *LocalStore) Exists(ctx context.Context, key string) (bool, error) {
	path := filepath.Join(s.basePath, key)
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// List lists objects with a prefix.
func (s *LocalStore) List(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	var objects []ObjectInfo

	err := filepath.Walk(s.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(s.basePath, path)
		if err != nil {
			return err
		}

		if strings.HasPrefix(relPath, prefix) {
			objects = append(objects, ObjectInfo{
				Key:          relPath,
				Size:         info.Size(),
				ContentType:  "application/octet-stream",
				LastModified: info.ModTime(),
			})
		}

		return nil
	})

	return objects, err
}

// URL returns a local file path (not a real URL).
func (s *LocalStore) URL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	return filepath.Join(s.basePath, key), nil
}

// ─── Log Store ────────────────────────────────────────────────────────────────

// LogStore handles job log storage.
type LogStore struct {
	store Store
}

// NewLogStore creates a new log store.
func NewLogStore(store Store) *LogStore {
	return &LogStore{store: store}
}

// LogChunk represents a chunk of log output.
type LogChunk struct {
	Sequence int64
	Content  string
	Source   string
}

// SaveLogChunk saves a log chunk to storage.
func (s *LogStore) SaveLogChunk(ctx context.Context, jobID string, chunk LogChunk) error {
	key := fmt.Sprintf("logs/%s/%06d.log", jobID, chunk.Sequence)
	data := strings.NewReader(chunk.Content)
	return s.store.Put(ctx, key, data, int64(len(chunk.Content)), "text/plain")
}

// GetLogChunks retrieves log chunks for a job.
func (s *LogStore) GetLogChunks(ctx context.Context, jobID string) ([]LogChunk, error) {
	prefix := fmt.Sprintf("logs/%s/", jobID)
	objects, err := s.store.List(ctx, prefix)
	if err != nil {
		return nil, err
	}

	var chunks []LogChunk
	for _, obj := range objects {
		// Extract sequence from key
		parts := strings.Split(obj.Key, "/")
		if len(parts) < 3 {
			continue
		}

		var seq int64
		fmt.Sscanf(parts[len(parts)-1], "%d.log", &seq)

		// Read content
		rc, err := s.store.Get(ctx, obj.Key)
		if err != nil {
			continue
		}

		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			continue
		}

		chunks = append(chunks, LogChunk{
			Sequence: seq,
			Content:  string(data),
			Source:   "stdout",
		})
	}

	return chunks, nil
}

// SaveJobLog saves a complete job log.
func (s *LogStore) SaveJobLog(ctx context.Context, jobID string, content string) error {
	key := fmt.Sprintf("logs/%s/complete.log", jobID)
	data := strings.NewReader(content)
	return s.store.Put(ctx, key, data, int64(len(content)), "text/plain")
}

// GetJobLog retrieves a complete job log.
func (s *LogStore) GetJobLog(ctx context.Context, jobID string) (string, error) {
	key := fmt.Sprintf("logs/%s/complete.log", jobID)
	rc, err := s.store.Get(ctx, key)
	if err != nil {
		return "", err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// ─── Artifact Store ───────────────────────────────────────────────────────────

// ArtifactStore handles project artifact storage.
type ArtifactStore struct {
	store Store
}

// NewArtifactStore creates a new artifact store.
func NewArtifactStore(store Store) *ArtifactStore {
	return &ArtifactStore{store: store}
}

// SaveArtifact saves a project artifact.
func (s *ArtifactStore) SaveArtifact(ctx context.Context, orgID, projectID, commitSHA string, data io.Reader, size int64) (string, error) {
	key := fmt.Sprintf("artifacts/%s/%s/%s.tar.gz", orgID, projectID, commitSHA)
	if err := s.store.Put(ctx, key, data, size, "application/gzip"); err != nil {
		return "", err
	}
	return key, nil
}

// GetArtifact retrieves a project artifact.
func (s *ArtifactStore) GetArtifact(ctx context.Context, artifactKey string) (io.ReadCloser, error) {
	return s.store.Get(ctx, artifactKey)
}

// DeleteArtifact deletes a project artifact.
func (s *ArtifactStore) DeleteArtifact(ctx context.Context, artifactKey string) error {
	return s.store.Delete(ctx, artifactKey)
}

// ListProjectArtifacts lists all artifacts for a project.
func (s *ArtifactStore) ListProjectArtifacts(ctx context.Context, orgID, projectID string) ([]ObjectInfo, error) {
	prefix := fmt.Sprintf("artifacts/%s/%s/", orgID, projectID)
	return s.store.List(ctx, prefix)
}

// CleanupOldArtifacts removes artifacts older than the retention period.
func (s *ArtifactStore) CleanupOldArtifacts(ctx context.Context, orgID, projectID string, retention time.Duration) (int, error) {
	prefix := fmt.Sprintf("artifacts/%s/%s/", orgID, projectID)
	objects, err := s.store.List(ctx, prefix)
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-retention)
	deleted := 0

	for _, obj := range objects {
		if obj.LastModified.Before(cutoff) {
			if err := s.store.Delete(ctx, obj.Key); err == nil {
				deleted++
			}
		}
	}

	return deleted, nil
}
