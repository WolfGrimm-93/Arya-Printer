package apiserver

import (
	"testing"

	"aryaescpos/internal/contract"
)

func TestValidatePrintRequest_Windows(t *testing.T) {
	req := contract.PrintRequest{Type: "windows", Content: "hi"}
	if err := ValidatePrintRequest(&req); err == nil {
		t.Fatal("expected error: printer_name is required for type=windows")
	} else if err.HTTPStatus != 400 {
		t.Fatalf("HTTPStatus = %d, want 400", err.HTTPStatus)
	}

	req = contract.PrintRequest{Type: "windows", Content: "hi", PrinterName: "HP"}
	if err := ValidatePrintRequest(&req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.ImageWidth != 576 {
		t.Errorf("ImageWidth default = %d, want 576", req.ImageWidth)
	}
}

func TestValidatePrintRequest_USB(t *testing.T) {
	req := contract.PrintRequest{Type: "usb", Content: "hi"}
	if err := ValidatePrintRequest(&req); err == nil {
		t.Fatal("expected error: vid/pid required for type=usb")
	}

	req = contract.PrintRequest{Type: "usb", Content: "hi", VID: "not-hex", PID: "0202"}
	if err := ValidatePrintRequest(&req); err == nil {
		t.Fatal("expected error: vid must be valid hex")
	}

	req = contract.PrintRequest{Type: "usb", Content: "hi", VID: "04b8", PID: "0202"}
	if err := ValidatePrintRequest(&req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePrintRequest_Network(t *testing.T) {
	req := contract.PrintRequest{Type: "network", Content: "hi"}
	if err := ValidatePrintRequest(&req); err == nil {
		t.Fatal("expected error: ip required for type=network")
	}

	req = contract.PrintRequest{Type: "network", Content: "hi", IP: "192.168.1.50"}
	if err := ValidatePrintRequest(&req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Port != 9100 {
		t.Errorf("Port default = %d, want 9100", req.Port)
	}
}

func TestValidatePrintRequest_Serial(t *testing.T) {
	req := contract.PrintRequest{Type: "serial", Content: "hi"}
	if err := ValidatePrintRequest(&req); err == nil {
		t.Fatal("expected error: com_port required for type=serial")
	}

	req = contract.PrintRequest{Type: "serial", Content: "hi", ComPort: "COM3"}
	if err := ValidatePrintRequest(&req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePrintRequest_BluetoothRejected(t *testing.T) {
	req := contract.PrintRequest{Type: "bluetooth", Content: "hi"}
	err := ValidatePrintRequest(&req)
	if err == nil {
		t.Fatal("expected bluetooth to be rejected")
	}
	if err.HTTPStatus != 400 {
		t.Fatalf("HTTPStatus = %d, want 400", err.HTTPStatus)
	}
}

func TestValidatePrintRequest_InvalidType(t *testing.T) {
	req := contract.PrintRequest{Type: "carrier-pigeon", Content: "hi"}
	if err := ValidatePrintRequest(&req); err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestValidateMatrixPrintRequest_Windows(t *testing.T) {
	req := contract.MatrixPrintRequest{Type: "windows", Content: "hi"}
	if err := ValidateMatrixPrintRequest(&req); err == nil {
		t.Fatal("expected error: printer_name required for type=windows")
	}

	req = contract.MatrixPrintRequest{Type: "windows", Content: "hi", PrinterName: "HP"}
	if err := ValidateMatrixPrintRequest(&req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Encoding != "cp850" {
		t.Errorf("Encoding default = %q, want cp850", req.Encoding)
	}
	if req.Font != "roman" {
		t.Errorf("Font default = %q, want roman", req.Font)
	}
	if req.CPI != 10 {
		t.Errorf("CPI default = %d, want 10", req.CPI)
	}
	if req.FormFeed == nil || !*req.FormFeed {
		t.Errorf("FormFeed default = %v, want true", req.FormFeed)
	}
	if req.BaudRate != 9600 {
		t.Errorf("BaudRate default = %d, want 9600", req.BaudRate)
	}
}

func TestValidateMatrixPrintRequest_NetworkNotAllowed(t *testing.T) {
	req := contract.MatrixPrintRequest{Type: "network", Content: "hi"}
	if err := ValidateMatrixPrintRequest(&req); err == nil {
		t.Fatal("expected error: network is not a valid matrix type")
	}
}

func TestValidateMatrixPrintRequest_USB(t *testing.T) {
	req := contract.MatrixPrintRequest{Type: "usb", Content: "hi", VID: "04b8", PID: "0202"}
	if err := ValidateMatrixPrintRequest(&req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateMatrixPrintRequest_Serial(t *testing.T) {
	req := contract.MatrixPrintRequest{Type: "serial", Content: "hi"}
	if err := ValidateMatrixPrintRequest(&req); err == nil {
		t.Fatal("expected error: com_port required for type=serial")
	}
}

func TestValidateMatrixPrintRequest_InvalidFont(t *testing.T) {
	req := contract.MatrixPrintRequest{Type: "windows", Content: "hi", PrinterName: "HP", Font: "comic-sans"}
	if err := ValidateMatrixPrintRequest(&req); err == nil {
		t.Fatal("expected error: invalid font")
	}
}

func TestValidateMatrixPrintRequest_InvalidCPI(t *testing.T) {
	req := contract.MatrixPrintRequest{Type: "windows", Content: "hi", PrinterName: "HP", CPI: 99}
	if err := ValidateMatrixPrintRequest(&req); err == nil {
		t.Fatal("expected error: invalid cpi")
	}
}

func TestValidateMatrixPrintRequest_BarcodeTypeNotValidatedHere(t *testing.T) {
	// Unknown barcode types must NOT fail validation here — they fall back
	// to a plain-text label downstream (contract.BarcodeItem doc comment).
	req := contract.MatrixPrintRequest{
		Type:        "windows",
		Content:     "hi",
		PrinterName: "HP",
		Barcodes:    []contract.BarcodeItem{{Data: "12345", Type: "made-up-symbology"}},
	}
	if err := ValidateMatrixPrintRequest(&req); err != nil {
		t.Fatalf("unexpected error for unknown barcode type: %v", err)
	}
}

func TestCenterText(t *testing.T) {
	got := centerText("hi", 6)
	if got != "  hi  " {
		t.Errorf("centerText = %q, want %q", got, "  hi  ")
	}
	// Odd padding: extra space goes on the right, matching Python's str.center.
	got = centerText("hi", 7)
	if got != "  hi   " {
		t.Errorf("centerText = %q, want %q", got, "  hi   ")
	}
}

func TestHumanFileSize(t *testing.T) {
	cases := []struct {
		size int64
		want string
	}{
		{500, "500 bytes"},
		{2048, "2.00 KB"},
		{5 * 1024 * 1024, "5.00 MB"},
	}
	for _, c := range cases {
		if got := humanFileSize(c.size); got != c.want {
			t.Errorf("humanFileSize(%d) = %q, want %q", c.size, got, c.want)
		}
	}
}
