package model

import (
	"crypto/rand"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

var (
	entropy     *ulid.MonotonicEntropy
	entropyOnce sync.Once
)

func getEntropy() *ulid.MonotonicEntropy {
	entropyOnce.Do(func() {
		entropy = ulid.Monotonic(rand.Reader, 0)
	})
	return entropy
}

func NewDownloadID() string {
	id := ulid.MustNew(ulid.Timestamp(time.Now()), getEntropy())
	return "d_" + id.String()
}
