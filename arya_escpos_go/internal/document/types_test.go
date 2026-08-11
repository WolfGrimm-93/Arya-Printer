package document

import "testing"

func TestDetectType(t *testing.T) {
	cases := []struct {
		filename string
		want     Type
	}{
		{"invoice.pdf", TypePDF},
		{"INVOICE.PDF", TypePDF},
		{"report.doc", TypeWord},
		{"report.docx", TypeWord},
		{"sheet.xls", TypeExcel},
		{"sheet.xlsx", TypeExcel},
		{"photo.jpg", TypeImage},
		{"photo.jpeg", TypeImage},
		{"photo.png", TypeImage},
		{"photo.bmp", TypeImage},
		{"photo.gif", TypeImage},
		{"photo.tiff", TypeImage},
		{"notes.txt", TypeText},
		{"app.log", TypeText},
		{"data.csv", TypeText},
		{"archive.zip", TypeUnknown},
		{"noextension", TypeUnknown},
		{"", TypeUnknown},
		{"weird.DoCx", TypeWord}, // case-insensitive extension match
	}

	for _, c := range cases {
		if got := DetectType(c.filename); got != c.want {
			t.Errorf("DetectType(%q) = %q, want %q", c.filename, got, c.want)
		}
	}
}
