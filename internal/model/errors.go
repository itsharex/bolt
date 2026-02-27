package model

import "errors"

var (
	ErrNotFound         = errors.New("download not found")
	ErrAlreadyActive    = errors.New("download is already active")
	ErrAlreadyPaused    = errors.New("download is already paused")
	ErrAlreadyCompleted = errors.New("download is already completed")
	ErrInvalidURL       = errors.New("invalid URL")
	ErrInvalidSegments  = errors.New("invalid segment count")
	ErrMaxRetriesExceeded = errors.New("maximum retries exceeded")
	ErrSizeMismatch     = errors.New("content length does not match original download size")
	ErrProbeRejected    = errors.New("server rejected probe request")
	ErrDuplicateURL     = errors.New("download with this URL already exists")
)
