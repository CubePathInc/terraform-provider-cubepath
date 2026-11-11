package client

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

const testBMUUID = "0f8e7d6c-5b4a-4392-8171-6f5e4d3c2b1a"

func TestParseBaremetalRef(t *testing.T) {
	good := map[string]string{
		"123":                       "123",
		testBMUUID:                  testBMUUID,
		strings.ToUpper(testBMUUID): testBMUUID,
		"  " + testBMUUID + " ":     testBMUUID,
	}
	for in, want := range good {
		got, err := ParseBaremetalRef(in)
		if err != nil || got != want {
			t.Errorf("ParseBaremetalRef(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "abc", "12a", "-1", "1.5", "../x", "123/power", "0f8e7d6c5b4a439281716f5e4d3c2b1a", testBMUUID + "0", "g" + testBMUUID[1:]} {
		if got, err := ParseBaremetalRef(in); err == nil {
			t.Errorf("ParseBaremetalRef(%q) = %q, want an error", in, got)
		}
	}
}

func TestBaremetalDecodesNumberOrString(t *testing.T) {
	cases := map[string]string{
		`{"id":123}`:                             "123",
		`{"id":"123"}`:                           "123",
		`{"id":123,"uuid":"` + testBMUUID + `"}`: "123",
		`{"id":null}`:                            "",
		`{"id":"` + testBMUUID + `"}`:            testBMUUID,
	}
	for in, want := range cases {
		var bm Baremetal
		if err := json.Unmarshal([]byte(in), &bm); err != nil || string(bm.ID) != want {
			t.Errorf("%s -> %q, %v; want %q", in, bm.ID, err, want)
		}
	}
	var bm Baremetal
	_ = json.Unmarshal([]byte(`{"id":1}`), &bm)
	if bm.UUID != "" {
		t.Errorf("uuid should be empty when absent, got %q", bm.UUID)
	}
}

func TestBaremetalGetByNumberOrUUID(t *testing.T) {
	for name, body := range map[string]string{
		"old shape": `[{"project":{"id":7},"baremetals":[{"id":5,"hostname":"a"},{"id":123,"hostname":"b"}]}]`,
		"new shape": `[{"project":{"id":7},"baremetals":[{"id":5,"uuid":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","hostname":"a"},{"id":"123","uuid":"` + strings.ToUpper(testBMUUID) + `","hostname":"b"}]}]`,
	} {
		t.Run(name, func(t *testing.T) {
			c, reqs := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) })
			bm, err := c.Baremetal.Get(context.Background(), "123")
			if err != nil || bm.Hostname != "b" || bm.ProjectID != 7 {
				t.Fatalf("by number: %+v, %v", bm, err)
			}
			if name == "new shape" {
				bm, err = c.Baremetal.Get(context.Background(), strings.ToUpper(testBMUUID))
				if err != nil || bm.Hostname != "b" {
					t.Fatalf("by uuid: %+v, %v", bm, err)
				}
			} else if _, err = c.Baremetal.Get(context.Background(), testBMUUID); err == nil {
				t.Fatal("old API has no uuid: want not found")
			}
			if _, err = c.Baremetal.Get(context.Background(), "999"); err == nil {
				t.Fatal("want not found")
			}
			n := len(*reqs)
			if _, err = c.Baremetal.Get(context.Background(), "nope"); err == nil || len(*reqs) != n {
				t.Fatalf("an invalid reference must fail before any request: %v, %d new requests", err, len(*reqs)-n)
			}
		})
	}
}

func TestBaremetalRoutesUseTheReference(t *testing.T) {
	c, reqs := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"detail":"ok"}`)) })
	ctx := context.Background()
	upper := strings.ToUpper(testBMUUID)
	for _, ref := range []string{"42", upper} {
		_ = c.Baremetal.Update(ctx, ref, &UpdateBaremetalRequest{})
		_, _ = c.Baremetal.Start(ctx, ref)
		_ = c.Baremetal.UpdateMonitoring(ctx, ref, true)
		_ = c.Baremetal.SetProtection(ctx, ref, true)
		_ = c.Baremetal.MoveProject(ctx, ref, 3)
		_ = c.Baremetal.AddSSHKeys(ctx, ref, []int{1})
		_ = c.Baremetal.RemoveSSHKey(ctx, ref, 1)
		_ = c.Baremetal.AttachNetwork(ctx, ref, 2)
		_ = c.Baremetal.DetachNetwork(ctx, ref)
		_, _ = c.Baremetal.Reinstall(ctx, ref, &ReinstallBaremetalRequest{})
		_, _ = c.Baremetal.Rescue(ctx, ref)
		_, _ = c.Baremetal.ResetBMC(ctx, ref)
	}
	want := []string{"update/%s", "%s/power/start_metal", "%s/monitoring", "%s/protection", "%s/move-project", "%s/ssh-keys", "%s/ssh-keys/1", "%s/network", "%s/network", "%s/reinstall", "%s/rescue", "%s/reset-bmc"}
	if len(*reqs) != 2*len(want) {
		t.Fatalf("%d requests", len(*reqs))
	}
	for i, r := range *reqs {
		ref := "42"
		if i >= len(want) {
			ref = testBMUUID
		}
		w := "/baremetal/" + strings.Replace(want[i%len(want)], "%s", ref, 1)
		if strings.HasPrefix(want[i%len(want)], "update/") {
			w = "/baremetal/update/" + ref
		}
		if r.Path != w {
			t.Errorf("request %d: %s, want %s", i, r.Path, w)
		}
	}
	n := len(*reqs)
	if err := c.Baremetal.SetProtection(ctx, "../vps/1", true); err == nil || len(*reqs) != n {
		t.Fatal("an invalid reference must fail before any request")
	}
}
