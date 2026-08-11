package winspool

import "fmt"

// PRINTER_STATUS_* bit values, from the Windows SDK's winspool.h. These are
// not exposed by golang.org/x/sys/windows, so they are reproduced here
// verbatim (values are part of the stable public Win32 ABI).
const (
	printerStatusPaused           = 0x00000001
	printerStatusError            = 0x00000002
	printerStatusPendingDeletion  = 0x00000004
	printerStatusPaperJam         = 0x00000008
	printerStatusPaperOut         = 0x00000010
	printerStatusPaperProblem     = 0x00000040
	printerStatusOffline          = 0x00000080
	printerStatusBusy             = 0x00000200
	printerStatusPrinting         = 0x00000400
	printerStatusNotAvailable     = 0x00001000
	printerStatusWaiting          = 0x00002000
	printerStatusProcessing       = 0x00004000
	printerStatusInitializing     = 0x00008000
	printerStatusWarmingUp        = 0x00010000
	printerStatusTonerLow         = 0x00020000
	printerStatusNoToner          = 0x00040000
	printerStatusPagePunt         = 0x00080000
	printerStatusUserIntervention = 0x00100000
	printerStatusOutOfMemory      = 0x00200000
	printerStatusDoorOpen         = 0x00400000
)

type statusFlag struct {
	flag  uint32
	label string
}

// The three tables below, and their iteration order, mirror
// device_routes.py's _decode_win_status() exactly (Python dicts preserve
// insertion order, which that function relies on implicitly when building
// the errors/warnings/details lists).
var errorFlags = []statusFlag{
	{printerStatusOffline, "Offline"},
	{printerStatusPaperJam, "Paper jam"},
	{printerStatusPaperOut, "Out of paper"},
	{printerStatusDoorOpen, "Door open"},
	{printerStatusError, "Printer error"},
	{printerStatusNotAvailable, "Not available"},
	{printerStatusOutOfMemory, "Out of memory"},
	{printerStatusNoToner, "No toner"},
}

var warningFlags = []statusFlag{
	{printerStatusTonerLow, "Toner low"},
	{printerStatusPagePunt, "Page punt"},
	{printerStatusUserIntervention, "User intervention required"},
	{printerStatusPaperProblem, "Paper problem"},
	{printerStatusPaused, "Paused"},
	{printerStatusPendingDeletion, "Pending deletion"},
}

var detailFlags = []statusFlag{
	{printerStatusBusy, "Busy"},
	{printerStatusPrinting, "Printing"},
	{printerStatusWarmingUp, "Warming up"},
	{printerStatusInitializing, "Initializing"},
	{printerStatusWaiting, "Waiting"},
	{printerStatusProcessing, "Processing"},
}

// decodeWinStatus decodes a PRINTER_INFO_2.Status bitfield into
// errors/warnings/details string lists, matching
// device_routes.py's _decode_win_status() bit-for-bit and label-for-label.
func decodeWinStatus(status uint32) (errs, warnings, details []string) {
	for _, f := range errorFlags {
		if status&f.flag != 0 {
			errs = append(errs, f.label)
		}
	}
	for _, f := range warningFlags {
		if status&f.flag != 0 {
			warnings = append(warnings, f.label)
		}
	}
	for _, f := range detailFlags {
		if status&f.flag != 0 {
			details = append(details, f.label)
		}
	}
	return errs, warnings, details
}

// jobStatusMap mirrors _get_windows_job_status()'s status_map: it maps the
// *raw* JOB_INFO_1.Status DWORD directly to a label, not the real spooler
// bitmask semantics (JOB_STATUS_* are bit flags, not a 0-6 enum) — this is
// a preserved, deliberate contract-fidelity quirk of the Python service,
// not a bug to fix here. See contract.JobStatus's doc comment.
var jobStatusMap = map[int]string{
	0: "queued",
	1: "paused",
	2: "error",
	3: "deleting",
	4: "spooling",
	5: "printing",
	6: "printed",
}

func decodeJobStatus(code int) string {
	if s, ok := jobStatusMap[code]; ok {
		return s
	}
	return fmt.Sprintf("unknown (%d)", code)
}
