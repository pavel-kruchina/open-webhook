// Package files provides on-disk storage for files extracted from captured multipart/form-data
// request bodies. File contents live on the local filesystem (never in the request storage); only
// file metadata is kept alongside the request. Files are grouped per session so that all files of a
// session can be removed at once when the session is gone.
package files

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a requested file does not exist on disk.
var ErrNotFound = errors.New("file not found")

// Storage stores file contents extracted from captured requests.
type Storage interface {
	// Create stores the content read from src as the file identified by fileUUID within the given session.
	// It returns the number of bytes written.
	Create(ctx context.Context, sID, fileUUID string, src io.Reader) (size int64, err error)

	// Open opens the file identified by fileUUID within the given session for reading.
	// If the file does not exist, ErrNotFound is returned.
	Open(ctx context.Context, sID, fileUUID string) (io.ReadCloser, error)

	// DeleteSession removes all files stored for the given session.
	DeleteSession(ctx context.Context, sID string) error
}

// Local is a filesystem-backed Storage implementation. Files are stored at <root>/<session-uuid>/<file-uuid>.
type Local struct {
	root string
}

var _ Storage = (*Local)(nil) // ensure interface implementation

// NewLocal creates a new filesystem-backed file storage rooted at the given directory.
func NewLocal(root string) *Local { return &Local{root: root} }

// validateIDs ensures both identifiers are well-formed UUIDs. This guards against path traversal:
// neither value can contain path separators or "..".
func validateIDs(ids ...string) error {
	for _, id := range ids {
		if _, err := uuid.Parse(id); err != nil {
			return fmt.Errorf("invalid identifier %q: %w", id, err)
		}
	}

	return nil
}

func (l *Local) sessionDir(sID string) string { return filepath.Join(l.root, sID) }

func (l *Local) filePath(sID, fileUUID string) string { return filepath.Join(l.root, sID, fileUUID) }

func (l *Local) Create(ctx context.Context, sID, fileUUID string, src io.Reader) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	if err := validateIDs(sID, fileUUID); err != nil {
		return 0, err
	}

	if err := os.MkdirAll(l.sessionDir(sID), 0o755); err != nil { //nolint:mnd
		return 0, fmt.Errorf("failed to create session directory: %w", err)
	}

	f, err := os.OpenFile(l.filePath(sID, fileUUID), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644) //nolint:mnd
	if err != nil {
		return 0, fmt.Errorf("failed to create file: %w", err)
	}

	defer func() { _ = f.Close() }()

	size, cErr := io.Copy(f, src)
	if cErr != nil {
		_ = os.Remove(l.filePath(sID, fileUUID)) // best-effort cleanup of the partial file

		return 0, fmt.Errorf("failed to write file: %w", cErr)
	}

	return size, nil
}

func (l *Local) Open(ctx context.Context, sID, fileUUID string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if err := validateIDs(sID, fileUUID); err != nil {
		return nil, err
	}

	f, err := os.Open(l.filePath(sID, fileUUID))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return f, nil
}

func (l *Local) DeleteSession(ctx context.Context, sID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := validateIDs(sID); err != nil {
		return err
	}

	return os.RemoveAll(l.sessionDir(sID))
}

// CleanupOrphans removes session directories whose session no longer exists, according to the provided
// exists function. It is safe to call periodically to reclaim disk space left behind by expired or
// deleted sessions (filesystem files are not subject to the storage TTL on their own).
func (l *Local) CleanupOrphans(ctx context.Context, exists func(ctx context.Context, sID string) (bool, error)) error {
	entries, err := os.ReadDir(l.root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return err
	}

	for _, e := range entries {
		if err = ctx.Err(); err != nil {
			return err
		}

		if !e.IsDir() {
			continue
		}

		var sID = e.Name()
		if _, pErr := uuid.Parse(sID); pErr != nil {
			continue // not a session directory we manage
		}

		ok, eErr := exists(ctx, sID)
		if eErr != nil {
			continue // on error, keep the directory to avoid deleting live data
		}

		if !ok {
			_ = os.RemoveAll(l.sessionDir(sID)) // best-effort
		}
	}

	return nil
}
