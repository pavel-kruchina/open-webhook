package files

import (
	"context"
	"time"
)

// OrphanCleaner removes orphaned session directories (see Local.CleanupOrphans).
type OrphanCleaner interface {
	CleanupOrphans(ctx context.Context, exists func(ctx context.Context, sID string) (bool, error)) error
}

// RunJanitor periodically removes files belonging to sessions that no longer exist, until the context
// is canceled. It runs one cleanup pass immediately and then once per interval. The exists callback
// must report whether a session is still present in the request storage.
func RunJanitor(
	ctx context.Context,
	cleaner OrphanCleaner,
	interval time.Duration,
	exists func(ctx context.Context, sID string) (bool, error),
) {
	var ticker = time.NewTicker(interval)
	defer ticker.Stop()

	_ = cleaner.CleanupOrphans(ctx, exists)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = cleaner.CleanupOrphans(ctx, exists)
		}
	}
}
