// Package api adalah kontrak modul identity: satu-satunya bagian yang boleh
// dipakai modul lain. Package ini tidak boleh mengimport ./internal.
package api

import "context"

// Greeting adalah hasil dari Greeter.
type Greeting struct {
	Message string `json:"message"`
}

// Greeter adalah kontrak yang harus diimplementasi identity service (./internal).
// Modul lain cukup bergantung pada interface ini, bukan pada implementasinya.
type Greeter interface {
	Greet(ctx context.Context, name string) (Greeting, error)
}
