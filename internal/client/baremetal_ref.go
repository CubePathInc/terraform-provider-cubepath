package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// FlexID is an identifier that the API sends as a JSON number today and may send as a
// string later. It keeps the text form, so a number and a numeric string decode to the
// same value and a value that is not a number never breaks the decoding of a listing.
type FlexID string

// UnmarshalJSON accepts a number, a string or null.
func (f *FlexID) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || string(data) == "null" {
		*f = ""
		return nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*f = FlexID(strings.TrimSpace(s))
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err != nil {
		return fmt.Errorf("id must be a number or a string: %w", err)
	}
	*f = FlexID(n.String())
	return nil
}

// MarshalJSON writes a numeric id as a number and anything else as a string.
func (f FlexID) MarshalJSON() ([]byte, error) {
	if baremetalNumberRe.MatchString(string(f)) {
		return []byte(f), nil
	}
	return json.Marshal(string(f))
}

// String returns the text form of the identifier.
func (f FlexID) String() string { return string(f) }

var (
	baremetalNumberRe = regexp.MustCompile(`^[0-9]+$`)
	baremetalUUIDRe   = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

// IsUUID reports whether s has the 8-4-4-4-12 hexadecimal shape of a uuid.
func IsUUID(s string) bool { return baremetalUUIDRe.MatchString(s) }

// ParseBaremetalRef validates a baremetal server reference: the server number (digits
// only) or its uuid (8-4-4-4-12 hexadecimal, any case). A number is returned as given and
// a uuid in lower case. Anything else is an error, so no request is made with it.
func ParseBaremetalRef(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	switch {
	case baremetalNumberRe.MatchString(ref):
		return ref, nil
	case baremetalUUIDRe.MatchString(ref):
		return strings.ToLower(ref), nil
	}
	return "", fmt.Errorf("invalid baremetal reference %q: use the server number or its uuid (8-4-4-4-12 hexadecimal)", ref)
}

// matchesBaremetalRef reports whether a server listed by the API is the one named by a
// reference already validated with ParseBaremetalRef.
func matchesBaremetalRef(bm *Baremetal, ref string) bool {
	return string(bm.ID) == ref || (bm.UUID != "" && strings.EqualFold(bm.UUID, ref))
}
