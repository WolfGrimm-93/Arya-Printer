package printsvc

import (
	"context"
	"fmt"

	"aryaescpos/internal/contract"
	"aryaescpos/internal/escpos"
)

type ticketPrinter struct {
	factory contract.AdapterFactory
	wq      contract.WindowsPrintQueue
}

// NewTicketPrinter returns a contract.TicketPrinter that builds ESC/POS
// command bytes via internal/escpos and sends them through factory (for
// usb/network/serial) or wq (for windows), replicating print_routes.py's
// POST /print handler: init -> (optional header image, centered) -> content
// -> three trailing newlines -> full cut. Encoding is always CP850,
// matching CommandBuilder(encoding="cp850") in the Python service.
func NewTicketPrinter(factory contract.AdapterFactory, wq contract.WindowsPrintQueue) contract.TicketPrinter {
	return &ticketPrinter{factory: factory, wq: wq}
}

func (t *ticketPrinter) Print(ctx context.Context, req contract.PrintRequest) (int, error) {
	builder := escpos.New()
	builder.Init()

	if req.HeaderImage != "" {
		if err := builder.Align(escpos.AlignCenter); err != nil {
			return 0, err
		}
		imgWidth := req.ImageWidth
		if imgWidth <= 0 {
			imgWidth = escpos.DefaultMaxWidth
		}
		if err := builder.Image(req.HeaderImage, imgWidth); err != nil {
			return 0, contract.BadRequest("invalid_header_image", err.Error())
		}
		if err := builder.Align(escpos.AlignLeft); err != nil {
			return 0, err
		}
		builder.Newline(1)
	}

	if err := builder.Text(req.Content); err != nil {
		return 0, contract.BadRequest("invalid_content_encoding", err.Error())
	}
	if err := builder.Text("\n\n\n"); err != nil {
		return 0, contract.BadRequest("invalid_content_encoding", err.Error())
	}
	builder.Cut(false)

	data := builder.Bytes()

	if err := t.send(ctx, req, data); err != nil {
		return 0, err
	}
	return len(data), nil
}

func (t *ticketPrinter) send(ctx context.Context, req contract.PrintRequest, data []byte) error {
	switch req.Type {
	case "windows":
		return t.wq.RawPrint(ctx, req.PrinterName, data, "")

	case "usb":
		vid, err := parseHexUint16(req.VID)
		if err != nil {
			return contract.BadRequest("invalid_vid", err.Error())
		}
		pid, err := parseHexUint16(req.PID)
		if err != nil {
			return contract.BadRequest("invalid_pid", err.Error())
		}
		return t.writeVia(ctx, "usb", contract.OpenParams{VID: vid, PID: pid}, data)

	case "network":
		port := req.Port
		if port == 0 {
			port = 9100
		}
		return t.writeVia(ctx, "network", contract.OpenParams{Host: req.IP, Port: port}, data)

	case "serial":
		// PrintRequest carries no baud_rate field (unlike MatrixPrintRequest);
		// the adapter's own default (9600, matching the Python service's
		// SerialAdapter constructor default) applies here.
		return t.writeVia(ctx, "serial", contract.OpenParams{ComPort: req.ComPort}, data)

	default:
		return contract.BadRequest("unsupported_device_type", fmt.Sprintf("unsupported device type: %s", req.Type))
	}
}

func (t *ticketPrinter) writeVia(ctx context.Context, deviceType string, params contract.OpenParams, data []byte) error {
	adapter, err := t.factory.Create(ctx, deviceType, params)
	if err != nil {
		return err
	}
	defer adapter.Close()
	_, err = adapter.Write(data)
	return err
}
