package storage

import (
	"context"
	"strings"
	"testing"
)

func TestLocalStore_PutGet(t *testing.T) {
	basePath := t.TempDir()
	store, err := NewLocalStore(basePath)
	if err != nil {
		t.Fatalf("NewLocalStore failed: %v", err)
	}

	ctx := context.Background()
	key := "test/data.txt"
	content := "Hello, World!"

	err = store.Put(ctx, key, strings.NewReader(content), int64(len(content)), "text/plain")
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	rc, err := store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	defer rc.Close()

	data := make([]byte, 1024)
	n, _ := rc.Read(data)
	if string(data[:n]) != content {
		t.Errorf("Get = %q, want %q", data[:n], content)
	}
}

func TestLocalStore_Exists(t *testing.T) {
	basePath := t.TempDir()
	store, _ := NewLocalStore(basePath)
	ctx := context.Background()

	key := "test/exists.txt"
	store.Put(ctx, key, strings.NewReader("data"), 4, "text/plain")

	exists, err := store.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("Exists should return true")
	}

	exists, _ = store.Exists(ctx, "nonexistent.txt")
	if exists {
		t.Error("Exists should return false")
	}
}

func TestLocalStore_Delete(t *testing.T) {
	basePath := t.TempDir()
	store, _ := NewLocalStore(basePath)
	ctx := context.Background()

	key := "test/delete.txt"
	store.Put(ctx, key, strings.NewReader("data"), 4, "text/plain")

	err := store.Delete(ctx, key)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	exists, _ := store.Exists(ctx, key)
	if exists {
		t.Error("Object should be deleted")
	}
}

func TestLogStore_SaveGet(t *testing.T) {
	basePath := t.TempDir()
	localStore, _ := NewLocalStore(basePath)
	logStore := NewLogStore(localStore)
	ctx := context.Background()

	jobID := "test-job-123"
	chunk := LogChunk{
		Sequence: 1,
		Content:  "PLAY [test]",
		Source:   "stdout",
	}

	err := logStore.SaveLogChunk(ctx, jobID, chunk)
	if err != nil {
		t.Fatalf("SaveLogChunk failed: %v", err)
	}

	chunks, err := logStore.GetLogChunks(ctx, jobID)
	if err != nil {
		t.Fatalf("GetLogChunks failed: %v", err)
	}

	if len(chunks) != 1 {
		t.Errorf("chunks length = %d, want 1", len(chunks))
	}
	if chunks[0].Content != "PLAY [test]" {
		t.Errorf("content = %q, want %q", chunks[0].Content, "PLAY [test]")
	}
}

func TestArtifactStore_SaveGet(t *testing.T) {
	basePath := t.TempDir()
	localStore, _ := NewLocalStore(basePath)
	artifactStore := NewArtifactStore(localStore)
	ctx := context.Background()

	orgID := "org-123"
	projectID := "proj-456"
	commitSHA := "abc123"

	key, err := artifactStore.SaveArtifact(ctx, orgID, projectID, commitSHA,
		strings.NewReader("artifact data"), 13)
	if err != nil {
		t.Fatalf("SaveArtifact failed: %v", err)
	}

	if !strings.Contains(key, commitSHA) {
		t.Errorf("key %q should contain commit SHA", key)
	}

	rc, err := artifactStore.GetArtifact(ctx, key)
	if err != nil {
		t.Fatalf("GetArtifact failed: %v", err)
	}
	defer rc.Close()

	data := make([]byte, 1024)
	n, _ := rc.Read(data)
	if string(data[:n]) != "artifact data" {
		t.Errorf("content = %q, want %q", data[:n], "artifact data")
	}
}
