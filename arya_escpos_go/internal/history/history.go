// Package history implements contract.HistoryStore: an in-memory, thread-safe
// print job history capped at 500 entries, newest first, with no persistence
// across restarts — a direct port of the Python service's
// utils/print_history.py (deque(maxlen=500) + a lock).
package history

import (
	"strings"
	"sync"
	"time"

	"aryaescpos/internal/contract"
)

// MaxEntries is the maximum number of history entries retained, matching the
// Python service's deque(maxlen=500).
const MaxEntries = 500

// Store is a thread-safe, in-memory, fixed-capacity print history buffer.
// The zero value is not usable; construct with NewStore.
type Store struct {
	mu      sync.Mutex
	entries []contract.HistoryEntry // index 0 is newest
}

// NewStore returns an empty history store.
func NewStore() *Store {
	return &Store{}
}

// Record assigns ID (epoch milliseconds), Timestamp (local time, seconds
// precision, matching Python's datetime.now().isoformat(timespec="seconds"))
// and Status ("sent") on entry, inserts it at the front, and trims the store
// to MaxEntries if needed. Safe for concurrent use.
//
// ID/Timestamp are stamped while holding the lock, immediately before
// insertion, specifically so that under concurrent callers the newest-first
// insertion order always agrees with ID/Timestamp order (time.Now() is
// monotonic non-decreasing on a single machine). Stamping them before
// acquiring the lock — the way the Python original computes its dict first
// and only locks for the appendleft — would let two goroutines race between
// "compute timestamp" and "acquire lock", so whichever happened to win the
// lock could end up inserted out of chronological order.
func (s *Store) Record(entry contract.HistoryEntry) contract.HistoryEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	entry.ID = now.UnixMilli()
	entry.Timestamp = now.Format("2006-01-02T15:04:05")
	entry.Status = "sent"

	s.entries = append(s.entries, contract.HistoryEntry{})
	copy(s.entries[1:], s.entries)
	s.entries[0] = entry
	if len(s.entries) > MaxEntries {
		s.entries = s.entries[:MaxEntries]
	}

	return entry
}

// Query returns at most limit entries, newest first, optionally filtered by
// an exact (not substring), case-insensitive match on PrinterName. limit is
// clamped to [1, MaxEntries] here as well, defensively, even though the
// contract documents the caller (apiserver) as responsible for clamping —
// clamping twice is harmless and keeps this method safe to call directly
// (e.g. from tests) without relying on caller discipline.
func (s *Store) Query(limit int, printerName string) []contract.HistoryEntry {
	if limit < 1 {
		limit = 1
	} else if limit > MaxEntries {
		limit = MaxEntries
	}

	s.mu.Lock()
	items := make([]contract.HistoryEntry, len(s.entries))
	copy(items, s.entries)
	s.mu.Unlock()

	if printerName != "" {
		filtered := items[:0]
		for _, e := range items {
			if strings.EqualFold(e.PrinterName, printerName) {
				filtered = append(filtered, e)
			}
		}
		items = filtered
	}

	if len(items) > limit {
		items = items[:limit]
	}
	return items
}
