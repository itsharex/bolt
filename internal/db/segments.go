package db

import (
	"context"
	"fmt"

	"github.com/fhsinchy/bolt/internal/model"
)

// InsertSegments batch-inserts a slice of segments into the database.
func (s *Store) InsertSegments(ctx context.Context, segments []model.Segment) error {
	if len(segments) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO segments (download_id, idx, start_byte, end_byte, downloaded, done)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare insert segment: %w", err)
	}
	defer stmt.Close()

	for _, seg := range segments {
		done := 0
		if seg.Done {
			done = 1
		}
		if _, err := stmt.ExecContext(ctx,
			seg.DownloadID, seg.Index, seg.StartByte, seg.EndByte,
			seg.Downloaded, done,
		); err != nil {
			return fmt.Errorf("insert segment %d: %w", seg.Index, err)
		}
	}

	return tx.Commit()
}

// GetSegments retrieves all segments for a download, ordered by index.
func (s *Store) GetSegments(ctx context.Context, downloadID string) ([]model.Segment, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT download_id, idx, start_byte, end_byte, downloaded, done
		FROM segments
		WHERE download_id = ?
		ORDER BY idx ASC`, downloadID)
	if err != nil {
		return nil, fmt.Errorf("query segments: %w", err)
	}
	defer rows.Close()

	var segments []model.Segment
	for rows.Next() {
		var seg model.Segment
		var done int
		if err := rows.Scan(
			&seg.DownloadID, &seg.Index, &seg.StartByte, &seg.EndByte,
			&seg.Downloaded, &done,
		); err != nil {
			return nil, fmt.Errorf("scan segment: %w", err)
		}
		seg.Done = done != 0
		segments = append(segments, seg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate segments: %w", err)
	}
	return segments, nil
}

// BatchUpdateSegments updates multiple segments in a single transaction
// using a prepared statement for efficiency.
func (s *Store) BatchUpdateSegments(ctx context.Context, segments []model.Segment) error {
	if len(segments) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		UPDATE segments
		SET downloaded = ?, done = ?
		WHERE download_id = ? AND idx = ?`)
	if err != nil {
		return fmt.Errorf("prepare update segment: %w", err)
	}
	defer stmt.Close()

	for _, seg := range segments {
		done := 0
		if seg.Done {
			done = 1
		}
		if _, err := stmt.ExecContext(ctx,
			seg.Downloaded, done, seg.DownloadID, seg.Index,
		); err != nil {
			return fmt.Errorf("update segment %d: %w", seg.Index, err)
		}
	}

	return tx.Commit()
}

// DeleteSegments deletes all segments for a given download ID.
func (s *Store) DeleteSegments(ctx context.Context, downloadID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM segments WHERE download_id = ?`, downloadID)
	if err != nil {
		return fmt.Errorf("delete segments: %w", err)
	}
	return nil
}
