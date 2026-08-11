// Package document converts non-PDF documents (Word, Excel, images, plain
// text) to PDF and prints the resulting PDF to a Windows printer.
//
// Security note (this package is the fix for a CRITICAL audit finding):
// the Python service this replaces converted DOCX/XLS uploads to PDF via
// real COM automation of Word/Excel (win32com), running as SYSTEM. An
// untrusted upload delivered to that path has no Mark-of-the-Web, so
// embedded macros could execute with SYSTEM privileges. This package
// converts Word/Excel exclusively via LibreOffice headless (soffice.exe,
// see convert_libreoffice.go), which removes the dependency on a real
// Office/COM instance entirely. IMPORTANT: --headless by itself does NOT
// disable macro execution — soffice --convert-to is the exact vector behind
// CVE-2018-16858, so the guarantee here does not come from --headless.
// It comes from also passing -env:UserInstallation pointing at an isolated
// profile whose registrymodifications.xcu forces
// Security/Scripting/MacroSecurityLevel=3 ("very high": never run a macro
// without a trusted signature, and no trusted sources are configured in
// this profile, so in practice none run). If that profile setup ever fails
// or is bypassed, this package's macro-execution surface is NOT reduced to
// zero by --headless alone — do not reintroduce COM automation here, and do
// not assume --headless is sufficient on its own.
//
// PDF printing (pdf_print.go) is done exclusively via PDFtoPrinter.exe
// (preferred, honors /copies) or SumatraPDF.exe (fallback, honors
// orientation). Neither external tool exposes control over color/duplex —
// both print using the target printer's current default settings for those
// two options. This is not a new regression: the Python service had the
// exact same partial support in its own PDFtoPrinter/SumatraPDF fallback
// path; here it is simply the only path instead of a last-resort one (no
// CGO/native rendering is used, by design).
package document
