package document

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif" // registers the .gif decoder with image.Decode
	"image/jpeg"
	_ "image/png" // registers the .png decoder with image.Decode
	"os"

	_ "golang.org/x/image/bmp"  // registers the .bmp decoder with image.Decode
	_ "golang.org/x/image/tiff" // registers the .tiff decoder with image.Decode

	"github.com/go-pdf/fpdf"

	"aryaescpos/internal/contract"
)

// imagePDFDPI matches the resolution the Python service passes to Pillow's
// Image.save(..., "PDF", resolution=300.0) when producing a full-page image
// PDF.
const imagePDFDPI = 300.0

// ConvertImageToPDF converts an image (jpg/jpeg/png/bmp/gif/tiff) at
// inputPath into a single-page PDF where the image fills the page at 300
// DPI, mirroring the Python service's _convert_image_to_pdf. .jpg/.jpeg,
// .png and .gif are decoded via the Go stdlib (image/jpeg, image/png,
// image/gif, all registered with image.Decode by their own packages);
// .bmp and .tiff need the golang.org/x/image/{bmp,tiff} decoders, imported
// here for their side-effecting registration only. Images with an alpha
// channel are composited onto an opaque white background (i.e. forced to
// RGB, losing transparency) — the same lossy behavior as Python's
// img.convert("RGB"), replicated intentionally for contract fidelity. The
// returned path is a temp file owned by the caller, who must remove it once
// done.
func ConvertImageToPDF(inputPath string) (string, error) {
	f, err := os.Open(inputPath)
	if err != nil {
		return "", contract.BadRequest("invalid_image", "No se pudo abrir la imagen: "+err.Error())
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return "", contract.BadRequest("invalid_image", "No se pudo decodificar la imagen: "+err.Error())
	}

	bounds := img.Bounds()
	wPx, hPx := bounds.Dx(), bounds.Dy()
	if wPx <= 0 || hPx <= 0 {
		return "", contract.BadRequest("invalid_image", "La imagen tiene dimensiones inválidas")
	}

	// Force RGB: composite over an opaque white background regardless of
	// whether the source has alpha, matching Pillow's img.convert("RGB").
	rgb := image.NewRGBA(bounds)
	draw.Draw(rgb, bounds, &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(rgb, bounds, img, bounds.Min, draw.Over)

	var jpegBuf bytes.Buffer
	if err := jpeg.Encode(&jpegBuf, rgb, &jpeg.Options{Quality: 92}); err != nil {
		return "", contract.BadGateway("image_conversion_failed", "No se pudo re-codificar la imagen", err.Error())
	}

	wPt := float64(wPx) / imagePDFDPI * 72.0
	hPt := float64(hPx) / imagePDFDPI * 72.0

	pdf := fpdf.NewCustom(&fpdf.InitType{
		OrientationStr: "P",
		UnitStr:        "pt",
		Size:           fpdf.SizeType{Wd: wPt, Ht: hPt},
	})
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddPage()
	imgOpts := fpdf.ImageOptions{ImageType: "JPG"}
	pdf.RegisterImageOptionsReader("img0", imgOpts, &jpegBuf)
	pdf.ImageOptions("img0", 0, 0, wPt, hPt, false, imgOpts, 0, "")

	if err := pdf.Error(); err != nil {
		return "", contract.BadGateway("image_conversion_failed", "No se pudo generar el PDF de la imagen", err.Error())
	}

	outPath, err := newTempPDFPath("aryaescpos-image")
	if err != nil {
		return "", contract.BadGateway("image_conversion_failed", "No se pudo reservar el archivo PDF de salida", err.Error())
	}
	if err := pdf.OutputFileAndClose(outPath); err != nil {
		os.Remove(outPath)
		return "", contract.BadGateway("image_conversion_failed", "No se pudo generar el PDF de la imagen", err.Error())
	}
	return outPath, nil
}
