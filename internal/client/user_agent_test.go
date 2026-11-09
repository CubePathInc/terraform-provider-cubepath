package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Every request carries terraform-provider-cubepath/<version> so the API logs
// can tell provider versions apart (Go would send Go-http-client/1.1).
func TestRequestsSendProviderUserAgent(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Header.Get("User-Agent"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()

	cases := []struct {
		name string
		opts []ClientOption
		want string
	}{
		{"with version", []ClientOption{WithMaxRetries(0), WithUserAgent("0.11.0")}, "terraform-provider-cubepath/0.11.0"},
		{"empty version keeps default", []ClientOption{WithMaxRetries(0), WithUserAgent("")}, "terraform-provider-cubepath/dev"},
		{"no option", []ClientOption{WithMaxRetries(0)}, "terraform-provider-cubepath/dev"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got = nil
			c, err := NewClient("token", srv.URL, tc.opts...)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := c.Projects.List(context.Background()); err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || got[0] != tc.want {
				t.Fatalf("User-Agent %v, want %q", got, tc.want)
			}
		})
	}
}
