// Package displayid generates and validates typed display IDs of the form
// `prefix-XXXXXXXXXXXX`, where the suffix is 12 random characters drawn
// from a 32-character Crockford base32 alphabet (~60 bits of entropy).
//
// Resource-specific prefixes (e.g. `cs` for changesets, `run` for runs)
// live with the corresponding model in `internal/model/`. This package is
// purely the formatting and validation utility and does not know about
// admiral's domains.
package displayid

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strings"
)

const suffixLen = 12

// alphabet is Crockford base32 in lowercase: digits 0-9 plus a-z excluding
// i, l, o, u. 32 characters with no visual look-alikes within the set.
const alphabet = "0123456789abcdefghjkmnpqrstvwxyz"

// Generate returns a typed display ID of the form `prefix-XXXXXXXXXXXX`.
// Callers must handle the rare unique-constraint collision by retrying.
func Generate(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Errorf("displayid: read random: %w", err))
	}
	n := binary.BigEndian.Uint64(b[:])
	suffix := make([]byte, suffixLen)
	for i := range suffixLen {
		suffix[i] = alphabet[n&31]
		n >>= 5
	}
	return prefix + "-" + string(suffix)
}

// Is reports whether s is a well-formed display ID for the given prefix.
// Used by lookup endpoints to dispatch between display-ID and UUID
// resolution without a regex match per request.
func Is(s, prefix string) bool {
	want := prefix + "-"
	if len(s) != len(want)+suffixLen {
		return false
	}
	if !strings.HasPrefix(s, want) {
		return false
	}
	for i := len(want); i < len(s); i++ {
		if !strings.ContainsRune(alphabet, rune(s[i])) {
			return false
		}
	}
	return true
}
