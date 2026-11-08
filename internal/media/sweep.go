package media

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/suwahadi/mabarbadmin/internal/database/sqlc"
)

type SweepResult struct {
	Rows  int
	Files int
	Bytes int64
}

// Sweep removes media rows that no owner references and files on disk without a media row.
// Both must be older than minAge so uploads whose transaction is still open are left alone.
func (s *Service) Sweep(ctx context.Context, q *sqlc.Queries, minAge time.Duration) (SweepResult, error) {
	var res SweepResult
	cutoff := time.Now().Add(-minAge)
	orphans, err := q.ListOrphanMedia(ctx, cutoff)
	if err != nil {
		return res, err
	}
	for _, m := range orphans {
		files, err := s.Delete(ctx, q, m.ID)
		if err != nil {
			continue
		}
		res.Rows++
		res.Bytes += m.Bytes
		res.Files += len(files)
		s.Purge(files)
	}

	rows, err := q.ListAllMediaKeys(ctx)
	if err != nil {
		return res, err
	}
	known := map[string]bool{}
	for _, r := range rows {
		known[r.StorageKey] = true
	}
	for _, root := range []string{s.PublicDir(), s.PrivateDir()} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return nil
				}
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return nil
			}
			if known[storageKeyFromFile(filepath.ToSlash(rel))] {
				return nil
			}
			info, err := d.Info()
			if err != nil || info.ModTime().After(cutoff) {
				return nil
			}
			if os.Remove(path) == nil {
				res.Files++
				res.Bytes += info.Size()
			}
			return nil
		})
		if err != nil {
			return res, err
		}
	}
	return res, nil
}

// storageKeyFromFile turns "ab/cd/<id>-640.webp" into "ab/cd/<id>".
func storageKeyFromFile(rel string) string {
	if i := strings.LastIndexByte(rel, '-'); i > 0 {
		return rel[:i]
	}
	return rel
}
