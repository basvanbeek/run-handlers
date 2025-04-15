package filewatcher

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/require"
)

func createTempFile(t *testing.T, dir, content string) string {
	t.Helper()
	file, err := os.CreateTemp(dir, "test-file-*.txt")
	require.NoError(t, err)
	_, err = file.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, file.Close())
	return file.Name()
}

func removeTempFile(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.Remove(path))
}

func initializeService(t *testing.T) *Service {
	t.Helper()
	svc := &Service{
		f: []*fileReg{},
		p: make(map[string]int),
	}
	_ = svc.Validate()
	_ = svc.PreRun()
	return svc
}

func TestNameReturnsCorrectValue(t *testing.T) {
	svc := initializeService(t)
	require.Equal(t, "file-watcher", svc.Name())
}

func TestAddWatcherRegistersFileSuccessfully(t *testing.T) {
	svc := initializeService(t)
	tempDir := t.TempDir()
	tempFile := createTempFile(t, tempDir, "initial content 1")
	defer removeTempFile(t, tempFile)

	ch, err := svc.AddWatcher("test-file", tempFile)
	require.NoError(t, err)
	require.NotNil(t, ch)
}

func TestAddWatcherFailsForDuplicateName(t *testing.T) {
	svc := initializeService(t)
	tempDir := t.TempDir()
	tempFile := createTempFile(t, tempDir, "initial content 2")
	defer removeTempFile(t, tempFile)

	_, err := svc.AddWatcher("test-file", tempFile)
	require.NoError(t, err)

	_, err = svc.AddWatcher("test-file", tempFile)
	require.Error(t, err)
	require.Contains(t, err.Error(), "registration already exists")
}

func TestRemoveWatcherRemovesFileSuccessfully(t *testing.T) {
	svc := initializeService(t)
	tempDir := t.TempDir()
	tempFile := createTempFile(t, tempDir, "initial content 3")
	defer removeTempFile(t, tempFile)

	_, err := svc.AddWatcher("test-file", tempFile)
	require.NoError(t, err)

	err = svc.RemoveWatcher("test-file")
	require.NoError(t, err)
}

func TestRemoveWatcherFailsForNonExistentName(t *testing.T) {
	svc := initializeService(t)
	err := svc.RemoveWatcher("nonexistent")
	require.Error(t, err)
	require.Contains(t, err.Error(), "registration nonexistent not found")
}

func TestServeContextProcessesFileEvents(t *testing.T) {
	svc := initializeService(t)
	tempDir := t.TempDir()
	tempFile := createTempFile(t, tempDir, "initial content 4")
	defer removeTempFile(t, tempFile)

	ch, err := svc.AddWatcher("test-file", tempFile)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err = svc.ServeContext(ctx)
		require.NoError(t, err)
	}()

	time.Sleep(100 * time.Millisecond)

	err = os.WriteFile(tempFile, []byte("updated content"), 0600)
	require.NoError(t, err)

	select {
	case data := <-ch:
		require.Equal(t, "updated content", string(data))
	case <-time.After(1 * time.Second):
		t.Fatal("file update event not received")
	}
}

func TestServeContextHandlesWatcherErrors(t *testing.T) {
	svc := initializeService(t)
	svc.w, _ = fsnotify.NewWatcher()
	defer svc.w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := svc.ServeContext(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "file watcher error channel closed unexpectedly")
	}()

	svc.w.Errors <- errors.New("simulated error")
	time.Sleep(100 * time.Millisecond)
}
