package printsvc

import (
	"context"
	"fmt"

	"aryaescpos/internal/contract"
	"aryaescpos/internal/escp"
)

type matrixPrinter struct {
	factory contract.AdapterFactory
	wq      contract.WindowsPrintQueue
}

// NewMatrixPrinter returns a contract.MatrixPrinter that builds ESC/P
// command bytes via internal/escp and sends them through factory (for
// usb/serial) or wq (for windows), replicating print_routes.py's
// POST /print/matrix handler: init -> font -> cpi -> line_spacing_sixth
// (always) -> content -> two trailing newlines -> each barcode in request
// order -> form feed (unless explicitly disabled).
func NewMatrixPrinter(factory contract.AdapterFactory, wq contract.WindowsPrintQueue) contract.MatrixPrinter {
	return &matrixPrinter{factory: factory, wq: wq}
}

func (m *matrixPrinter) Print(ctx context.Context, req contract.MatrixPrintRequest) (int, error) {
	encoding := req.Encoding
	if encoding == "" {
		encoding = "cp850"
	}
	builder, err := escp.New(encoding)
	if err != nil {
		return 0, contract.BadRequest("unsupported_encoding", err.Error())
	}

	font := req.Font
	if font == "" {
		font = escp.FontRoman
	}
	cpi := req.CPI
	if cpi == 0 {
		cpi = 10
	}

	builder.Init()
	builder.Font(font)
	builder.CPI(cpi)
	builder.LineSpacingSixth() // always called, unconditionally, matching the Python flow
	builder.Text(req.Content)
	builder.Newline(2)

	for _, bc := range req.Barcodes {
		builder.Barcode(bc)
	}

	formFeed := true
	if req.FormFeed != nil {
		formFeed = *req.FormFeed
	}
	if formFeed {
		builder.FormFeed()
	}

	data := builder.Bytes()

	if err := m.send(ctx, req, data); err != nil {
		return 0, err
	}
	return len(data), nil
}

func (m *matrixPrinter) send(ctx context.Context, req contract.MatrixPrintRequest, data []byte) error {
	switch req.Type {
	case "windows":
		return m.wq.RawPrint(ctx, req.PrinterName, data, "")

	case "usb":
		vid, err := parseHexUint16(req.VID)
		if err != nil {
			return contract.BadRequest("invalid_vid", err.Error())
		}
		pid, err := parseHexUint16(req.PID)
		if err != nil {
			return contract.BadRequest("invalid_pid", err.Error())
		}
		return m.writeVia(ctx, "usb", contract.OpenParams{VID: vid, PID: pid}, data)

	case "serial":
		baud := req.BaudRate
		if baud == 0 {
			baud = 9600
		}
		return m.writeVia(ctx, "serial", contract.OpenParams{ComPort: req.ComPort, BaudRate: baud}, data)

	default:
		// Matrix printing intentionally has no "network" or "bluetooth"
		// transport, matching MatrixPrintRequest.Type's documented enum.
		return contract.BadRequest("unsupported_device_type", fmt.Sprintf("unsupported device type: %s", req.Type))
	}
}

func (m *matrixPrinter) writeVia(ctx context.Context, deviceType string, params contract.OpenParams, data []byte) error {
	adapter, err := m.factory.Create(ctx, deviceType, params)
	if err != nil {
		return err
	}
	defer adapter.Close()
	_, err = adapter.Write(data)
	return err
}
