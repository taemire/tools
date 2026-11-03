package artexmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
"bytes"

	"golang.org/x/text/unicode/norm"
)

func Hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// CanonicalJSON uses sorted object keys, compact UTF-8 and NFC string values.
func CanonicalJSON(v any) ([]byte, error) {
	b, e := json.Marshal(v)
	if e != nil {
		return nil, e
	}
	var x any
	dec:=json.NewDecoder(bytes.NewReader(b));dec.UseNumber();if e = dec.Decode(&x); e != nil {
		return nil, e
	}
	var normalize func(any) any
	normalize = func(v any) any {
		switch a := v.(type) {
		case string:
			return norm.NFC.String(a)
		case []any:
			for i := range a {
				a[i] = normalize(a[i])
			}
			return a
		case map[string]any:
			r := map[string]any{}
			for k, w := range a {
				r[norm.NFC.String(k)] = normalize(w)
			}
			return r
		}
		return v
	}
	return json.Marshal(normalize(x))
}
func DetailHash(c Component) (string, error) {
	c.SHA256 = ""
	b, e := CanonicalJSON(c)
	return Hash(b), e
}

// SHA of the reviewed visual-token SSOT, not a second token JSON copy.
const VisualTokensSHA256 = "f85cb4058a6b165a15c7b917a2880579c97a7ec54f70e27457b77d1c054f3923"
