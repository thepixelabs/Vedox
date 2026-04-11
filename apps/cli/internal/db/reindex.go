package db

import (
	"context"
	"fmt"
)

// Reindex performs a full rebuild of the metadata/FTS index from the
// Markdown tree. It is the disaster-recovery path mandated by the
// Epic: `rm .vedox/index.db && vedox reindex` must restore a fully
// searchable workspace with zero data loss.
//
// The sequence is:
//  1. Truncate documents + FTS (serialised through the writer).
//  2. Walk every *.md under workspaceRoot via the DocStore adapter.
//  3. Upsert each discovered Doc.
//
// Truncate + rebuild is simpler and safer than diffing. At Phase 1
// document counts (low thousands) it completes in well under a second
// on a laptop SSD; if that ever stops being true, Reindex becomes the
// natural place to add incremental rebuilds keyed on content_hash.
func (s *Store) Reindex(ctx context.Context, store DocStore, workspaceRoot string) error {
	if store == nil {
		return fmt.Errorf("vedox: Reindex requires a DocStore")
	}
	if workspaceRoot == "" {
		workspaceRoot = s.workspaceRoot
	}
	if err := s.truncate(ctx); err != nil {
		return fmt.Errorf("vedox: reindex truncate: %w", err)
	}
	count := 0
	err := store.WalkDocs(workspaceRoot, func(d *Doc) error {
		if d == nil {
			return nil
		}
		if err := s.UpsertDoc(ctx, d); err != nil {
			return fmt.Errorf("vedox: reindex upsert %s: %w", d.ID, err)
		}
		count++
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}
