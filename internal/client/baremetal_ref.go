package client

import (
	"fmt"
	"regexp"
	"strings"
)

var baremetalUUIDRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// IsUUID reports whether s has the 8-4-4-4-12 hexadecimal shape of a uuid.
func IsUUID(s string) bool { return baremetalUUIDRe.MatchString(s) }

// ParseBaremetalRef validates a baremetal server reference: its uuid (8-4-4-4-12
// hexadecimal, any case), returned in lower case. Anything else, a server number included,
// is an error, so no request is made with it.
func ParseBaremetalRef(ref string) (string, error) {
	if IsUUID(ref) {
		return strings.ToLower(ref), nil
	}
	return "", fmt.Errorf("invalid baremetal reference %q: baremetal reference must be a uuid", ref)
}

// matchesBaremetalRef reports whether a server listed by the API is the one named by a
// reference already validated with ParseBaremetalRef.
func matchesBaremetalRef(bm *Baremetal, ref string) bool {
	return strings.EqualFold(bm.ID, ref)
}
