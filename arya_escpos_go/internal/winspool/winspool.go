// Package winspool wraps the Windows Print Spooler (winspool.drv) via
// golang.org/x/sys/windows.NewLazyDLL bindings (no CGO), implementing
// contract.WindowsPrintQueue. It replicates the behavior of
// arya_escpos/src/server/{adapter_factory,device_routes}.py's
// send_raw_to_windows / _scan_windows_printers / _get_windows_printer_status
// / _decode_win_status / _get_windows_job_status, and
// arya_escpos/src/utils/pdf_printer.py's get_job_id_from_queue.
package winspool

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"aryaescpos/internal/contract"
)

const (
	printerEnumLocal       = 0x00000002
	printerEnumConnections = 0x00000004
)

// --- Win32 struct layouts ------------------------------------------------
//
// These mirror the corresponding *_W (Unicode) structs in winspool.h field
// for field. winspool.h does not pack these structs, so ordinary C/Go
// natural alignment applies and no manual padding is required as long as
// field order and widths match.

// printerInfo1W mirrors PRINTER_INFO_1W, the level pywin32's
// win32print.EnumPrinters() returns by default (Flags, Description, Name,
// Comment) — matched here for parity with the Python service's tuple
// unpacking in _scan_windows_printers().
type printerInfo1W struct {
	Flags       uint32
	Description *uint16
	Name        *uint16
	Comment     *uint16
}

// printerInfo2W mirrors PRINTER_INFO_2W. Only Status is actually read by
// this package (via PrinterStatus); the rest of the fields exist purely to
// keep the struct's memory layout correct for GetPrinter(level=2).
type printerInfo2W struct {
	ServerName         *uint16
	PrinterName        *uint16
	ShareName          *uint16
	PortName           *uint16
	DriverName         *uint16
	Comment            *uint16
	Location           *uint16
	DevMode            unsafe.Pointer
	SepFile            *uint16
	PrintProcessor     *uint16
	Datatype           *uint16
	Parameters         *uint16
	SecurityDescriptor unsafe.Pointer
	Attributes         uint32
	Priority           uint32
	DefaultPriority    uint32
	StartTime          uint32
	UntilTime          uint32
	Status             uint32
	CJobs              uint32
	AveragePPM         uint32
}

// jobInfo1W mirrors JOB_INFO_1W.
type jobInfo1W struct {
	JobId        uint32
	PrinterName  *uint16
	MachineName  *uint16
	UserName     *uint16
	Document     *uint16
	Datatype     *uint16
	Status       *uint16
	StatusCode   uint32
	Priority     uint32
	Position     uint32
	TotalPages   uint32
	PagesPrinted uint32
	Submitted    windows.Systemtime
}

// docInfo1W mirrors DOC_INFO_1W, used by StartDocPrinter.
type docInfo1W struct {
	DocName    *uint16
	OutputFile *uint16
	Datatype   *uint16
}

// --- lazy DLL binding ------------------------------------------------------

var (
	winspoolDLL = windows.NewLazyDLL("winspool.drv")

	procOpenPrinterW     = winspoolDLL.NewProc("OpenPrinterW")
	procClosePrinter     = winspoolDLL.NewProc("ClosePrinter")
	procStartDocPrinterW = winspoolDLL.NewProc("StartDocPrinterW")
	procStartPagePrinter = winspoolDLL.NewProc("StartPagePrinter")
	procWritePrinter     = winspoolDLL.NewProc("WritePrinter")
	procEndPagePrinter   = winspoolDLL.NewProc("EndPagePrinter")
	procEndDocPrinter    = winspoolDLL.NewProc("EndDocPrinter")
	procEnumPrintersW    = winspoolDLL.NewProc("EnumPrintersW")
	procGetPrinterW      = winspoolDLL.NewProc("GetPrinterW")
	procEnumJobsW        = winspoolDLL.NewProc("EnumJobsW")
	procGetJobW          = winspoolDLL.NewProc("GetJobW")
)

// Queue implements contract.WindowsPrintQueue.
type Queue struct{}

// NewQueue returns a Queue backed by the real Windows Print Spooler.
func NewQueue() *Queue {
	return &Queue{}
}

func openPrinter(name string) (windows.Handle, error) {
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return 0, fmt.Errorf("winspool: invalid printer name %q: %w", name, err)
	}
	var h windows.Handle
	r1, _, e1 := procOpenPrinterW.Call(
		uintptr(unsafe.Pointer(namePtr)),
		uintptr(unsafe.Pointer(&h)),
		0,
	)
	if r1 == 0 {
		return 0, contract.NotFound("printer_not_found", fmt.Sprintf("OpenPrinter(%q) failed: %v", name, e1))
	}
	return h, nil
}

func closePrinter(h windows.Handle) {
	procClosePrinter.Call(uintptr(h))
}

// RawPrint sends data to printerName via the spooler's RAW datatype:
// OpenPrinter -> StartDocPrinter -> StartPagePrinter -> WritePrinter ->
// EndPagePrinter -> EndDocPrinter, with ClosePrinter always run on the way
// out (equivalent to Python's try/finally), matching
// adapter_factory.py's send_raw_to_windows() exactly.
func (q *Queue) RawPrint(ctx context.Context, printerName string, data []byte, jobName string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if jobName == "" {
		jobName = "Arya ESCPOS Print Job"
	}

	h, err := openPrinter(printerName)
	if err != nil {
		return err
	}
	defer closePrinter(h)

	jobNamePtr, err := windows.UTF16PtrFromString(jobName)
	if err != nil {
		return fmt.Errorf("winspool: invalid job name %q: %w", jobName, err)
	}
	datatypePtr, err := windows.UTF16PtrFromString("RAW")
	if err != nil {
		return err
	}
	docInfo := docInfo1W{DocName: jobNamePtr, OutputFile: nil, Datatype: datatypePtr}

	r1, _, e1 := procStartDocPrinterW.Call(uintptr(h), 1, uintptr(unsafe.Pointer(&docInfo)))
	if r1 == 0 {
		return contract.BadGateway("windows_start_doc_failed", fmt.Sprintf("StartDocPrinter(%q) failed", printerName), fmt.Sprint(e1))
	}

	r1, _, e1 = procStartPagePrinter.Call(uintptr(h))
	if r1 == 0 {
		return contract.BadGateway("windows_start_page_failed", fmt.Sprintf("StartPagePrinter(%q) failed", printerName), fmt.Sprint(e1))
	}

	var written uint32
	var dataPtr *byte
	if len(data) > 0 {
		dataPtr = &data[0]
	}
	r1, _, e1 = procWritePrinter.Call(uintptr(h), uintptr(unsafe.Pointer(dataPtr)), uintptr(len(data)), uintptr(unsafe.Pointer(&written)))
	if r1 == 0 {
		return contract.BadGateway("windows_write_failed", fmt.Sprintf("WritePrinter(%q) failed", printerName), fmt.Sprint(e1))
	}

	procEndPagePrinter.Call(uintptr(h))
	procEndDocPrinter.Call(uintptr(h))
	return nil
}

// ScanPrinters lists all local and connected Windows printers, matching
// _scan_windows_printers()'s id/type/name/manufacturer/description shape.
func (q *Queue) ScanPrinters(ctx context.Context) ([]contract.DeviceInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	const flags = printerEnumLocal | printerEnumConnections
	var needed, returned uint32
	procEnumPrintersW.Call(uintptr(flags), 0, 1, 0, 0, uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&returned)))
	if needed == 0 {
		return nil, nil
	}

	buf := make([]byte, needed)
	r1, _, e1 := procEnumPrintersW.Call(
		uintptr(flags), 0, 1,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(needed),
		uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&returned)),
	)
	if r1 == 0 {
		return nil, contract.BadGateway("windows_enum_printers_failed", "EnumPrinters failed", fmt.Sprint(e1))
	}

	entries := unsafe.Slice((*printerInfo1W)(unsafe.Pointer(&buf[0])), returned)
	devices := make([]contract.DeviceInfo, 0, returned)
	for _, e := range entries {
		name := windows.UTF16PtrToString(e.Name)
		description := windows.UTF16PtrToString(e.Description)
		comment := windows.UTF16PtrToString(e.Comment)

		connType := "network"
		if e.Flags&printerEnumLocal != 0 {
			connType = "local"
		}
		desc := connType
		if comment != "" {
			desc = connType + " - " + comment
		}

		devices = append(devices, contract.DeviceInfo{
			ID:           "win-" + strings.ReplaceAll(name, " ", "_"),
			Type:         "windows",
			Name:         name,
			Manufacturer: description,
			Description:  desc,
		})
	}
	return devices, nil
}

// PrinterStatus opens printerName, reads its PRINTER_INFO_2 status bitfield
// via GetPrinter(level=2), and decodes it exactly like
// _get_windows_printer_status()/_decode_win_status(). jobs_in_queue comes
// from EnumJobs and defaults to 0 if that call fails (never aborts the
// whole status check), same as the Python service's try/except.
func (q *Queue) PrinterStatus(ctx context.Context, printerName string) (contract.DeviceStatus, error) {
	if err := ctx.Err(); err != nil {
		return contract.DeviceStatus{}, err
	}

	h, err := openPrinter(printerName)
	if err != nil {
		return contract.DeviceStatus{}, err
	}
	defer closePrinter(h)

	var needed uint32
	procGetPrinterW.Call(uintptr(h), 2, 0, 0, uintptr(unsafe.Pointer(&needed)))
	if needed == 0 {
		return contract.DeviceStatus{}, contract.BadGateway("windows_get_printer_failed", fmt.Sprintf("GetPrinter(%q) failed to size buffer", printerName), "")
	}

	buf := make([]byte, needed)
	r1, _, e1 := procGetPrinterW.Call(uintptr(h), 2, uintptr(unsafe.Pointer(&buf[0])), uintptr(needed), uintptr(unsafe.Pointer(&needed)))
	if r1 == 0 {
		return contract.DeviceStatus{}, contract.BadGateway("windows_get_printer_failed", fmt.Sprintf("GetPrinter(%q) failed", printerName), fmt.Sprint(e1))
	}

	info := (*printerInfo2W)(unsafe.Pointer(&buf[0]))
	statusCode := int(info.Status)
	errs, warnings, details := decodeWinStatus(info.Status)
	isOnline := len(errs) == 0

	var statusText string
	switch {
	case len(errs) > 0:
		statusText = "Error: " + strings.Join(errs, ", ")
	case len(warnings) > 0:
		statusText = "Warning: " + strings.Join(warnings, ", ")
	case len(details) > 0:
		statusText = strings.Join(details, ", ")
	default:
		statusText = "Ready"
	}

	jobsInQueue := 0
	if n, jerr := countJobs(h); jerr == nil {
		jobsInQueue = n
	}

	return contract.DeviceStatus{
		DeviceType:  "windows",
		PrinterName: printerName,
		IsOnline:    isOnline,
		Status:      statusText,
		StatusCode:  &statusCode,
		Errors:      errs,
		Warnings:    warnings,
		Details:     details,
		JobsInQueue: &jobsInQueue,
	}, nil
}

func countJobs(h windows.Handle) (int, error) {
	var needed, returned uint32
	procEnumJobsW.Call(uintptr(h), 0, uintptr(0xFFFFFFFF), 1, 0, 0, uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&returned)))
	if needed == 0 {
		return 0, nil
	}
	buf := make([]byte, needed)
	r1, _, e1 := procEnumJobsW.Call(uintptr(h), 0, uintptr(0xFFFFFFFF), 1, uintptr(unsafe.Pointer(&buf[0])), uintptr(needed), uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&returned)))
	if r1 == 0 {
		return 0, fmt.Errorf("winspool: EnumJobs failed: %v", e1)
	}
	return int(returned), nil
}

// JobStatus fetches a single job's status via GetJob(level=1), mapping
// StatusCode through the same raw-code->label table as
// _get_windows_job_status()'s status_map (see status.go's doc comment on
// jobStatusMap for why this deliberately does not decode the real
// JOB_STATUS_* bitmask).
func (q *Queue) JobStatus(ctx context.Context, printerName string, jobID int) (contract.JobStatus, error) {
	if err := ctx.Err(); err != nil {
		return contract.JobStatus{}, err
	}

	h, err := openPrinter(printerName)
	if err != nil {
		return contract.JobStatus{}, err
	}
	defer closePrinter(h)

	var needed uint32
	procGetJobW.Call(uintptr(h), uintptr(jobID), 1, 0, 0, uintptr(unsafe.Pointer(&needed)))
	if needed == 0 {
		return contract.JobStatus{}, contract.NotFound("job_not_found", fmt.Sprintf("job %d not found on %q", jobID, printerName))
	}

	buf := make([]byte, needed)
	r1, _, e1 := procGetJobW.Call(uintptr(h), uintptr(jobID), 1, uintptr(unsafe.Pointer(&buf[0])), uintptr(needed), uintptr(unsafe.Pointer(&needed)))
	if r1 == 0 {
		return contract.JobStatus{}, contract.NotFound("job_not_found", fmt.Sprintf("job %d not found on %q or GetJob failed: %v", jobID, printerName, e1))
	}

	info := (*jobInfo1W)(unsafe.Pointer(&buf[0]))
	statusCode := int(info.StatusCode)
	submitted := info.Submitted

	return contract.JobStatus{
		JobID:           int(info.JobId),
		PrinterName:     printerName,
		DocumentName:    windows.UTF16PtrToString(info.Document),
		Status:          decodeJobStatus(statusCode),
		StatusCode:      statusCode,
		TotalPages:      int(info.TotalPages),
		PagesPrinted:    int(info.PagesPrinted),
		PositionInQueue: int(info.Position),
		SubmittedTime:   formatSystemtime(submitted),
		SizeInBytes:     0, // JOB_INFO_1 carries no Size field; JOB_INFO_2 would, but is not fetched here
	}, nil
}

func formatSystemtime(st windows.Systemtime) string {
	return fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d", st.Year, st.Month, st.Day, st.Hour, st.Minute, st.Second)
}

// FindRecentJob polls EnumJobs every 200ms until timeout looking for a job
// whose document name matches docName by exact name, basename, substring,
// or stem — the same matching logic (and order) as
// pdf_printer.py's get_job_id_from_queue(). Returns (nil, nil), not an
// error, if nothing matches within timeout.
func (q *Queue) FindRecentJob(ctx context.Context, printerName, docName string, timeout time.Duration) (*int, error) {
	h, err := openPrinter(printerName)
	if err != nil {
		return nil, nil //nolint:nilerr // mirrors Python: open failure just means "not found", not a caller-visible error
	}
	defer closePrinter(h)

	deadline := time.Now().Add(timeout)
	docLower := strings.ToLower(docName)
	baseLower := strings.ToLower(filepath.Base(docName))
	stemLower := strings.ToLower(strings.TrimSuffix(filepath.Base(docName), filepath.Ext(docName)))

	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if jobID, found := findJobOnce(h, docLower, baseLower, stemLower); found {
			return &jobID, nil
		}

		time.Sleep(200 * time.Millisecond)
	}
	return nil, nil
}

func findJobOnce(h windows.Handle, docLower, baseLower, stemLower string) (int, bool) {
	var needed, returned uint32
	procEnumJobsW.Call(uintptr(h), 0, uintptr(0xFFFFFFFF), 1, 0, 0, uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&returned)))
	if needed == 0 {
		return 0, false
	}
	buf := make([]byte, needed)
	r1, _, _ := procEnumJobsW.Call(uintptr(h), 0, uintptr(0xFFFFFFFF), 1, uintptr(unsafe.Pointer(&buf[0])), uintptr(needed), uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&returned)))
	if r1 == 0 {
		return 0, false
	}

	jobs := unsafe.Slice((*jobInfo1W)(unsafe.Pointer(&buf[0])), returned)
	for _, j := range jobs {
		jobDoc := strings.ToLower(windows.UTF16PtrToString(j.Document))
		if docLower == jobDoc ||
			baseLower == jobDoc ||
			strings.Contains(jobDoc, docLower) ||
			strings.Contains(jobDoc, baseLower) ||
			strings.Contains(jobDoc, stemLower) {
			return int(j.JobId), true
		}
	}
	return 0, false
}
