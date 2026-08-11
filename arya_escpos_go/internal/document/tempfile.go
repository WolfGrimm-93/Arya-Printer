package document

import "os"

// newTempPDFPath reserves a unique *.pdf path under the OS temp dir without
// leaving a stray empty file behind for writers (such as fpdf's
// OutputFileAndClose or LibreOffice's --outdir) that create/overwrite the
// file themselves.
func newTempPDFPath(prefix string) (string, error) {
	f, err := os.CreateTemp("", prefix+"-*.pdf")
	if err != nil {
		return "", err
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		return "", err
	}
	return name, nil
}
