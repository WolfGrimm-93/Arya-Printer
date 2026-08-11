package history

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"aryaescpos/internal/contract"
)

func TestRecordAssignsFieldsAndNewestFirst(t *testing.T) {
	s := NewStore()

	first := s.Record(contract.HistoryEntry{PrinterName: "LX-350", Document: "a.txt", Protocol: "escp", BytesSent: 10})
	second := s.Record(contract.HistoryEntry{PrinterName: "LX-350", Document: "b.txt", Protocol: "escp", BytesSent: 20})

	if first.Status != "sent" || second.Status != "sent" {
		t.Fatalf("Status = %q / %q, want %q", first.Status, second.Status, "sent")
	}
	if first.ID == 0 || second.ID == 0 {
		t.Fatalf("expected non-zero IDs, got %d / %d", first.ID, second.ID)
	}
	if first.Timestamp == "" || second.Timestamp == "" {
		t.Fatal("expected non-empty Timestamp")
	}

	got := s.Query(10, "")
	if len(got) != 2 {
		t.Fatalf("Query returned %d entries, want 2", len(got))
	}
	// Newest first: "b.txt" was recorded second, must come first.
	if got[0].Document != "b.txt" || got[1].Document != "a.txt" {
		t.Fatalf("order = [%s, %s], want [b.txt, a.txt] (newest first)", got[0].Document, got[1].Document)
	}
}

func TestQueryFilterIsExactCaseInsensitive(t *testing.T) {
	s := NewStore()
	s.Record(contract.HistoryEntry{PrinterName: "LX-350", Document: "a.txt"})
	s.Record(contract.HistoryEntry{PrinterName: "lx-350", Document: "b.txt"})
	s.Record(contract.HistoryEntry{PrinterName: "LX-350-XL", Document: "c.txt"})

	got := s.Query(10, "lx-350")
	if len(got) != 2 {
		t.Fatalf("Query(printerName=lx-350) returned %d entries, want 2 (exact case-insensitive match, not substring)", len(got))
	}
	for _, e := range got {
		if !strings.EqualFold(e.PrinterName, "lx-350") {
			t.Fatalf("unexpected entry with PrinterName=%q matched filter lx-350", e.PrinterName)
		}
	}
}

func TestQueryLimitClamping(t *testing.T) {
	s := NewStore()
	s.Record(contract.HistoryEntry{PrinterName: "P1"})
	s.Record(contract.HistoryEntry{PrinterName: "P1"})
	s.Record(contract.HistoryEntry{PrinterName: "P1"})

	if got := s.Query(0, ""); len(got) != 1 {
		t.Fatalf("Query(limit=0) returned %d entries, want 1 (clamped to minimum)", len(got))
	}
	if got := s.Query(-5, ""); len(got) != 1 {
		t.Fatalf("Query(limit=-5) returned %d entries, want 1 (clamped to minimum)", len(got))
	}
	if got := s.Query(2, ""); len(got) != 2 {
		t.Fatalf("Query(limit=2) returned %d entries, want 2", len(got))
	}
}

func TestRecordCapsAt500(t *testing.T) {
	s := NewStore()
	for i := 0; i < MaxEntries+50; i++ {
		s.Record(contract.HistoryEntry{PrinterName: "P1", Document: fmt.Sprintf("doc-%d", i)})
	}

	got := s.Query(MaxEntries, "")
	if len(got) != MaxEntries {
		t.Fatalf("stored %d entries, want capped at %d", len(got), MaxEntries)
	}
	// Newest first: the very last one recorded (doc-549) must be at index 0.
	want := fmt.Sprintf("doc-%d", MaxEntries+50-1)
	if got[0].Document != want {
		t.Fatalf("got[0].Document = %q, want %q (newest first after overflow)", got[0].Document, want)
	}
}

func TestRecordConcurrentWritesAreSafeAndOrdered(t *testing.T) {
	s := NewStore()
	const goroutines = 50
	const perGoroutine = 20

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				s.Record(contract.HistoryEntry{
					PrinterName: "Concurrent",
					Document:    fmt.Sprintf("g%d-i%d", g, i),
					Protocol:    "escpos",
					BytesSent:   i,
				})
			}
		}(g)
	}
	wg.Wait()

	total := goroutines * perGoroutine // 1000, exceeds MaxEntries
	got := s.Query(MaxEntries, "")
	if len(got) != MaxEntries {
		t.Fatalf("after %d concurrent writes, store has %d entries, want capped at %d", total, len(got), MaxEntries)
	}

	// Newest-first invariant: IDs must be non-increasing down the slice.
	for i := 1; i < len(got); i++ {
		if got[i].ID > got[i-1].ID {
			t.Fatalf("entries not newest-first at index %d: ID %d > previous ID %d", i, got[i].ID, got[i-1].ID)
		}
	}

	// Race-detector coverage of concurrent Query while writes are in flight.
	var wg2 sync.WaitGroup
	wg2.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(g int) {
			defer wg2.Done()
			s.Record(contract.HistoryEntry{PrinterName: "Concurrent2", Document: fmt.Sprintf("more-%d", g)})
			_ = s.Query(50, "Concurrent2")
		}(g)
	}
	wg2.Wait()
}
