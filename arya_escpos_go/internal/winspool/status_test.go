package winspool

import (
	"reflect"
	"testing"
)

func TestDecodeWinStatusReady(t *testing.T) {
	errs, warnings, details := decodeWinStatus(0)
	if len(errs) != 0 || len(warnings) != 0 || len(details) != 0 {
		t.Fatalf("decodeWinStatus(0) = %v, %v, %v; want all empty", errs, warnings, details)
	}
}

func TestDecodeWinStatusSingleError(t *testing.T) {
	errs, warnings, details := decodeWinStatus(printerStatusPaperOut)
	if !reflect.DeepEqual(errs, []string{"Out of paper"}) {
		t.Errorf("errs = %v, want [Out of paper]", errs)
	}
	if len(warnings) != 0 || len(details) != 0 {
		t.Errorf("warnings/details should be empty, got %v / %v", warnings, details)
	}
}

func TestDecodeWinStatusMultipleFlagsOrder(t *testing.T) {
	// Offline (error) + Paused (warning) + Printing (detail), combined into
	// one bitmask, must decode in the fixed table order regardless of bit
	// value ordering.
	status := uint32(printerStatusOffline | printerStatusPaused | printerStatusPrinting)
	errs, warnings, details := decodeWinStatus(status)
	if !reflect.DeepEqual(errs, []string{"Offline"}) {
		t.Errorf("errs = %v, want [Offline]", errs)
	}
	if !reflect.DeepEqual(warnings, []string{"Paused"}) {
		t.Errorf("warnings = %v, want [Paused]", warnings)
	}
	if !reflect.DeepEqual(details, []string{"Printing"}) {
		t.Errorf("details = %v, want [Printing]", details)
	}
}

func TestDecodeWinStatusOrderWithinCategory(t *testing.T) {
	// Paper out set together with Door open: table order is
	// Offline, Paper jam, Paper out, Door open, ... so Paper out must
	// precede Door open in the result regardless of numeric bit value.
	status := uint32(printerStatusDoorOpen | printerStatusPaperOut)
	errs, _, _ := decodeWinStatus(status)
	want := []string{"Out of paper", "Door open"}
	if !reflect.DeepEqual(errs, want) {
		t.Errorf("errs = %v, want %v", errs, want)
	}
}

func TestDecodeJobStatusKnownCodes(t *testing.T) {
	cases := map[int]string{
		0: "queued",
		1: "paused",
		2: "error",
		3: "deleting",
		4: "spooling",
		5: "printing",
		6: "printed",
	}
	for code, want := range cases {
		if got := decodeJobStatus(code); got != want {
			t.Errorf("decodeJobStatus(%d) = %q, want %q", code, got, want)
		}
	}
}

func TestDecodeJobStatusUnknownCode(t *testing.T) {
	if got := decodeJobStatus(99); got != "unknown (99)" {
		t.Errorf("decodeJobStatus(99) = %q, want \"unknown (99)\"", got)
	}
}
