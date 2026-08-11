// Package apiserver wires the 9 HTTP endpoints of the Arya ESCPOS agent on
// top of the contract.* interfaces. It depends on nothing but
// internal/contract (plus the standard library): concrete implementations
// (hwadapter, winspool, escpos, document, printsvc) are injected by
// cmd/aryaprinter/main.go via Deps, never imported here directly.
package apiserver

import "aryaescpos/internal/contract"

// Deps holds every dependency a handler needs, all typed as contract.*
// interfaces so this package stays decoupled from their implementations.
// main.go builds one Deps value from the concrete packages Agents B and C
// produce and passes it to New.
type Deps struct {
	Scanner    contract.DeviceScanner
	PrintQueue contract.WindowsPrintQueue
	Ticket     contract.TicketPrinter
	Matrix     contract.MatrixPrinter
	Document   contract.DocumentPrinter
	History    contract.HistoryStore

	// ConfigView is a JSON-serializable snapshot of the running
	// configuration for GET /api/v1/config. It is an opaque `any` rather
	// than config.Config so this package never has to import
	// internal/config: the caller (main.go) is responsible for building it
	// from config.Config and stripping every secret (Security.APIKeyPath
	// first and foremost) before handing it to New.
	ConfigView any

	// MaxImageBytes is Security.MaxImageBytes (config.types.go), passed in
	// as a plain int64 rather than a config.Config field for the same
	// dependency-boundary reason as ConfigView. handlePrint enforces it
	// against PrintRequest.HeaderImage's decoded size — 0 or negative
	// disables the check (treated as "no limit configured").
	MaxImageBytes int64
}
