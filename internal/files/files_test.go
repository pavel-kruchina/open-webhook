package files_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gh.tarampamp.am/webhook-tester/v2/internal/files"
)

func TestLocal_CreateOpenDelete(t *testing.T) {
	t.Parallel()

	var (
		ctx     = context.Background()
		root    = t.TempDir()
		store   = files.NewLocal(root)
		sID     = uuid.New().String()
		fID     = uuid.New().String()
		content = []byte("hello, this is a file payload")
	)

	size, err := store.Create(ctx, sID, fID, strings.NewReader(string(content)))
	require.NoError(t, err)
	assert.Equal(t, int64(len(content)), size)

	// the file must exist on disk at <root>/<sID>/<fID>
	assert.FileExists(t, filepath.Join(root, sID, fID))

	// and be readable back with the same content
	rc, oErr := store.Open(ctx, sID, fID)
	require.NoError(t, oErr)

	got, rErr := io.ReadAll(rc)
	require.NoError(t, rErr)
	require.NoError(t, rc.Close())
	assert.Equal(t, content, got)

	// deleting the session removes the file
	require.NoError(t, store.DeleteSession(ctx, sID))
	assert.NoDirExists(t, filepath.Join(root, sID))

	_, oErr = store.Open(ctx, sID, fID)
	assert.ErrorIs(t, oErr, files.ErrNotFound)
}

func TestLocal_OpenMissing(t *testing.T) {
	t.Parallel()

	var store = files.NewLocal(t.TempDir())

	_, err := store.Open(context.Background(), uuid.New().String(), uuid.New().String())
	assert.ErrorIs(t, err, files.ErrNotFound)
}

func TestLocal_RejectsNonUUIDIdentifiers(t *testing.T) {
	t.Parallel()

	var (
		ctx   = context.Background()
		store = files.NewLocal(t.TempDir())
	)

	for name, tt := range map[string]struct{ sID, fID string }{
		"path traversal in session": {sID: "../../etc", fID: uuid.New().String()},
		"path traversal in file":    {sID: uuid.New().String(), fID: "../../../etc/passwd"},
		"empty session":             {sID: "", fID: uuid.New().String()},
		"not a uuid":                {sID: "not-a-uuid", fID: "also-not"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, cErr := store.Create(ctx, tt.sID, tt.fID, strings.NewReader("x"))
			assert.Error(t, cErr)

			_, oErr := store.Open(ctx, tt.sID, tt.fID)
			assert.Error(t, oErr)
		})
	}
}

func TestLocal_CleanupOrphans(t *testing.T) {
	t.Parallel()

	var (
		ctx     = context.Background()
		root    = t.TempDir()
		store   = files.NewLocal(root)
		live    = uuid.New().String()
		orphan  = uuid.New().String()
		fileUID = uuid.New().String()
	)

	_, err := store.Create(ctx, live, fileUID, strings.NewReader("live"))
	require.NoError(t, err)
	_, err = store.Create(ctx, orphan, fileUID, strings.NewReader("orphan"))
	require.NoError(t, err)

	// a stray non-UUID directory must be left untouched
	require.NoError(t, os.MkdirAll(filepath.Join(root, "not-a-session"), 0o755))

	// only the live session exists in storage
	exists := func(_ context.Context, sID string) (bool, error) { return sID == live, nil }

	require.NoError(t, store.CleanupOrphans(ctx, exists))

	assert.DirExists(t, filepath.Join(root, live))
	assert.NoDirExists(t, filepath.Join(root, orphan))
	assert.DirExists(t, filepath.Join(root, "not-a-session"))
}
