package contract

// APIError is a typed error that internal/apiserver translates directly into
// an HTTP response of the shape {"detail": Message}. Detail carries extra
// diagnostic context for logs; apiserver decides whether/how much of it to
// surface to the client.
//
// HTTP status convention (apply consistently across all packages):
//   - 400 Bad Request:          malformed/invalid client input.
//   - 401 Unauthorized:         missing/invalid API key.
//   - 403 Forbidden:            request blocked by a security policy (e.g. netguard).
//   - 404 Not Found:            referenced device/job/printer does not exist.
//   - 413 Payload Too Large:    upload or decoded payload exceeds configured limit.
//   - 502 Bad Gateway:          talked to hardware or an external process (USB/serial/TCP
//     write, SumatraPDF/PDFtoPrinter, LibreOffice) and it failed.
//   - 503 Service Unavailable:  a required environment prerequisite is missing
//     (LibreOffice not installed, Sumatra/PDFtoPrinter not found anywhere,
//     Windows Printer subsystem unavailable).
type APIError struct {
	HTTPStatus int
	Code       string
	Message    string
	Detail     string
}

func (e *APIError) Error() string { return e.Message }

func BadRequest(code, msg string) *APIError {
	return &APIError{HTTPStatus: 400, Code: code, Message: msg}
}

func Unauthorized(msg string) *APIError {
	return &APIError{HTTPStatus: 401, Code: "unauthorized", Message: msg}
}

func Forbidden(code, msg string) *APIError {
	return &APIError{HTTPStatus: 403, Code: code, Message: msg}
}

func NotFound(code, msg string) *APIError {
	return &APIError{HTTPStatus: 404, Code: code, Message: msg}
}

func PayloadTooLarge(msg string) *APIError {
	return &APIError{HTTPStatus: 413, Code: "payload_too_large", Message: msg}
}

func BadGateway(code, msg, detail string) *APIError {
	return &APIError{HTTPStatus: 502, Code: code, Message: msg, Detail: detail}
}

func ServiceUnavailable(code, msg, detail string) *APIError {
	return &APIError{HTTPStatus: 503, Code: code, Message: msg, Detail: detail}
}
