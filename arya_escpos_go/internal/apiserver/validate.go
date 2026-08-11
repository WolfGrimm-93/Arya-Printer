package apiserver

import (
	"encoding/base64"
	"fmt"
	"strconv"

	"aryaescpos/internal/contract"
)

// maxRasterDimension bounds any client-supplied pixel/dot dimension that
// ends up sized into an in-memory raster (PrintRequest.ImageWidth,
// BarcodeItem.WidthDots/HeightDots). Without a cap, a client can send e.g.
// width_dots: 2000000000 and force a multi-gigabyte allocation in
// image.NewRGBA downstream (internal/escpos/image.go,
// internal/escp/barcode.go) before any real printer hardware is ever
// touched — a trivial single-request DoS. 4000 is generous for anything
// this agent prints (576 dots covers 80mm thermal paper at 203dpi; even a
// full-page 300dpi barcode graphic stays well under it) while being many
// orders of magnitude short of causing a problematic allocation.
const maxRasterDimension = 4000

// validateHex mirrors schemas.py's _validate_hex: an empty/absent value is
// fine (required-ness is checked separately, per type), but a present value
// must parse as hex.
func validateHex(value, fieldName string) *contract.APIError {
	if value == "" {
		return nil
	}
	if _, err := strconv.ParseUint(value, 16, 32); err != nil {
		return contract.BadRequest("invalid_hex", fieldName+" must be a valid hex string (e.g. '04b8'), got '"+value+"'")
	}
	return nil
}

// validateHeaderImageSize enforces Security.MaxImageBytes on
// PrintRequest.HeaderImage — config declared in config.types.go but never
// actually applied by any handler prior to this fix. It rejects both an
// oversized base64 string (before paying the cost of decoding it) and an
// oversized decoded payload, so a client can't force a large allocation via
// base64.DecodeString itself, and so the check runs before HeaderImage ever
// reaches image.Decode downstream in internal/escpos.
func validateHeaderImageSize(encoded string, maxBytes int64) *contract.APIError {
	if encoded == "" || maxBytes <= 0 {
		return nil
	}
	if len(encoded) > base64.StdEncoding.EncodedLen(int(maxBytes)) {
		return contract.PayloadTooLarge(fmt.Sprintf("header_image exceeds the configured max_image_bytes limit (%d bytes)", maxBytes))
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return contract.BadRequest("invalid_image", "header_image is not valid base64")
	}
	if int64(len(decoded)) > maxBytes {
		return contract.PayloadTooLarge(fmt.Sprintf("header_image exceeds the configured max_image_bytes limit (%d bytes)", maxBytes))
	}
	return nil
}

// applyPrintRequestDefaults mirrors PrintRequest's Pydantic field defaults
// (image_width=576, port=9100) for fields the client omitted (zero value).
func applyPrintRequestDefaults(req *contract.PrintRequest) {
	if req.ImageWidth == 0 {
		req.ImageWidth = 576
	}
	if req.Port == 0 {
		req.Port = 9100
	}
}

// ValidatePrintRequest mirrors schemas.py's PrintRequest validation:
// hex-format checks on vid/pid, then check_required_fields's per-type
// required fields. Bluetooth is accepted by the Type literal (kept for
// wire compatibility) but explicitly out of scope for this v1, unlike the
// Python service which actually printed over it.
func ValidatePrintRequest(req *contract.PrintRequest) *contract.APIError {
	switch req.Type {
	case "windows", "usb", "network", "serial":
	case "bluetooth":
		return contract.BadRequest("unsupported_type", "bluetooth is not supported in this version")
	default:
		return contract.BadRequest("invalid_type", "type must be one of: windows, usb, network, serial")
	}

	if err := validateHex(req.VID, "vid"); err != nil {
		return err
	}
	if err := validateHex(req.PID, "pid"); err != nil {
		return err
	}

	applyPrintRequestDefaults(req)

	// Reject an out-of-bounds image_width before it ever reaches
	// image.Decode/image.NewRGBA downstream — see maxRasterDimension.
	// ImageWidth is never 0 here (applyPrintRequestDefaults already turned
	// an omitted value into 576), so <=0 only happens for a client-supplied
	// negative number, which is just as invalid as one that's too large.
	if req.ImageWidth <= 0 || req.ImageWidth > maxRasterDimension {
		return contract.BadRequest("invalid_field", fmt.Sprintf("image_width must be between 1 and %d", maxRasterDimension))
	}

	switch req.Type {
	case "windows":
		if req.PrinterName == "" {
			return contract.BadRequest("missing_field", "printer_name is required for type='windows'")
		}
	case "usb":
		if req.VID == "" || req.PID == "" {
			return contract.BadRequest("missing_field", "vid and pid are required for type='usb'")
		}
	case "network":
		if req.IP == "" {
			return contract.BadRequest("missing_field", "ip is required for type='network'")
		}
	case "serial":
		if req.ComPort == "" {
			return contract.BadRequest("missing_field", "com_port is required for type='serial'")
		}
	}

	return nil
}

// applyMatrixDefaults mirrors MatrixPrintRequest's Pydantic field defaults
// for fields the client omitted.
func applyMatrixDefaults(req *contract.MatrixPrintRequest) {
	if req.Encoding == "" {
		req.Encoding = "cp850"
	}
	if req.FormFeed == nil {
		formFeed := true
		req.FormFeed = &formFeed
	}
	if req.Font == "" {
		req.Font = "roman"
	}
	if req.CPI == 0 {
		req.CPI = 10
	}
	if req.BaudRate == 0 {
		req.BaudRate = 9600
	}
	for i := range req.Barcodes {
		if req.Barcodes[i].Type == "" {
			req.Barcodes[i].Type = "code39"
		}
		if req.Barcodes[i].WidthDots == 0 {
			req.Barcodes[i].WidthDots = 300
		}
		if req.Barcodes[i].HeightDots == 0 {
			req.Barcodes[i].HeightDots = 48
		}
	}
}

// ValidateMatrixPrintRequest mirrors schemas.py's MatrixPrintRequest
// validation. Barcode Type is intentionally NOT restricted to a fixed enum
// here — per contract.BarcodeItem's doc comment, an unsupported/invalid
// barcode type must fall back to a plain-text label downstream rather than
// fail the whole request, so rejecting it at this layer would be wrong.
func ValidateMatrixPrintRequest(req *contract.MatrixPrintRequest) *contract.APIError {
	switch req.Type {
	case "windows", "usb", "serial":
	default:
		return contract.BadRequest("invalid_type", "type must be one of: windows, usb, serial")
	}

	if err := validateHex(req.VID, "vid"); err != nil {
		return err
	}
	if err := validateHex(req.PID, "pid"); err != nil {
		return err
	}

	applyMatrixDefaults(req)

	switch req.Font {
	case "roman", "sans_serif":
	default:
		return contract.BadRequest("invalid_field", "font must be one of: roman, sans_serif")
	}
	// cpi mirrors schemas.py's Literal[10, 12, 15]. Go's zero value (0) is
	// indistinguishable from "field omitted" at decode time, so
	// applyMatrixDefaults coerces exactly that value to 10 above — matching
	// what an omitted cpi means in the Python service. Anything else the
	// client actually sent that isn't one of the three allowed values
	// (e.g. cpi: 7) must be rejected outright, not silently coerced.
	if req.CPI != 10 && req.CPI != 12 && req.CPI != 15 {
		return contract.BadRequest("invalid_field", "cpi must be one of: 10, 12, 15")
	}

	for i, bc := range req.Barcodes {
		// Same DoS guard as image_width: bounds any client-supplied dot
		// dimension before it reaches a raster allocation downstream (see
		// maxRasterDimension). WidthDots/HeightDots are never 0 here
		// (applyMatrixDefaults already defaulted them), so <=0 only
		// happens for a client-supplied negative number.
		if bc.WidthDots <= 0 || bc.WidthDots > maxRasterDimension {
			return contract.BadRequest("invalid_field", fmt.Sprintf("barcodes[%d].width_dots must be between 1 and %d", i, maxRasterDimension))
		}
		if bc.HeightDots <= 0 || bc.HeightDots > maxRasterDimension {
			return contract.BadRequest("invalid_field", fmt.Sprintf("barcodes[%d].height_dots must be between 1 and %d", i, maxRasterDimension))
		}
	}

	switch req.Type {
	case "windows":
		if req.PrinterName == "" {
			return contract.BadRequest("missing_field", "printer_name is required for type='windows'")
		}
	case "usb":
		if req.VID == "" || req.PID == "" {
			return contract.BadRequest("missing_field", "vid and pid are required for type='usb'")
		}
	case "serial":
		if req.ComPort == "" {
			return contract.BadRequest("missing_field", "com_port is required for type='serial'")
		}
	}

	return nil
}
