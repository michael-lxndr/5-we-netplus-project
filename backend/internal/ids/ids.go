// Package ids genera identificadores únicos (UUID v4) para los registros.
package ids

import (
	"crypto/rand"
	"fmt"
)

// New devuelve un UUID v4 en texto.
func New() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // sin entropía del sistema no se puede operar con seguridad
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
