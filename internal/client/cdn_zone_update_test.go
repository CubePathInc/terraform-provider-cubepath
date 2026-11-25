package client

import (
	"encoding/json"
	"testing"
)

func TestUpdateCDNZoneRequestCacheTagHeader(t *testing.T) {
	name, empty, on := "Cache-Tag", "", true
	cases := []struct {
		name string
		req  UpdateCDNZoneRequest
		want map[string]any
	}{
		{"untouched", UpdateCDNZoneRequest{}, map[string]any{}},
		{"set", UpdateCDNZoneRequest{CacheTagHeader: &name}, map[string]any{"cache_tag_header": "Cache-Tag"}},
		{"cleared with null", UpdateCDNZoneRequest{CacheTagHeader: &empty}, map[string]any{"cache_tag_header": nil}},
		{"compression", UpdateCDNZoneRequest{CompressionEnabled: &on}, map[string]any{"compression_enabled": true}},
	}
	for _, tc := range cases {
		body, err := json.Marshal(tc.req)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		var got map[string]any
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if len(got) != len(tc.want) {
			t.Fatalf("%s: got %s", tc.name, body)
		}
		for k, v := range tc.want {
			gv, ok := got[k]
			if !ok || gv != v {
				t.Fatalf("%s: field %s = %v (present %v), want %v", tc.name, k, gv, ok, v)
			}
		}
	}
}
