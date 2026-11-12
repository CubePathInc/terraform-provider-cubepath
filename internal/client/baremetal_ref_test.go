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
		testBMUUID:                  testBMUUID,
		strings.ToUpper(testBMUUID): testBMUUID,
	}
	for in, want := range good {
		got, err := ParseBaremetalRef(in)
		if err != nil || got != want {
			t.Errorf("ParseBaremetalRef(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "123", "007", "abc", "12a", "-1", "1.5", "../x", "123/power", "0f8e7d6c5b4a439281716f5e4d3c2b1a", testBMUUID + "0", "g" + testBMUUID[1:], " " + testBMUUID} {
		got, err := ParseBaremetalRef(in)
		if err == nil {
			t.Errorf("ParseBaremetalRef(%q) = %q, want an error", in, got)
		} else if !strings.Contains(err.Error(), "baremetal reference must be a uuid") {
			t.Errorf("ParseBaremetalRef(%q): %v", in, err)
		}
	}
}

func TestBaremetalDecodesUUIDID(t *testing.T) {
	var bm Baremetal
	if err := json.Unmarshal([]byte(`{"id":"`+testBMUUID+`","uuid":"`+testBMUUID+`"}`), &bm); err != nil || bm.ID != testBMUUID || bm.UUID != testBMUUID {
		t.Fatalf("%+v %v", bm, err)
	}
}

func TestBaremetalGetByUUID(t *testing.T) {
	other := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	body := `[{"project":{"id":7},"baremetals":[{"id":"` + other + `","uuid":"` + other + `","hostname":"a"},{"id":"` + testBMUUID + `","uuid":"` + testBMUUID + `","hostname":"b"}]}]`
	c, reqs := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) })
	bm, err := c.Baremetal.Get(context.Background(), strings.ToUpper(testBMUUID))
	if err != nil || bm.Hostname != "b" || bm.ProjectID != 7 {
		t.Fatalf("by uuid: %+v, %v", bm, err)
	}
	if _, err = c.Baremetal.Get(context.Background(), "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"); err == nil {
		t.Fatal("want not found")
	} else if apiErr, ok := err.(*APIError); !ok || !apiErr.IsNotFound() {
		t.Fatalf("want a 404 APIError, got %v", err)
	}
	n := len(*reqs)
	for _, bad := range []string{"123", "nope"} {
		if _, err = c.Baremetal.Get(context.Background(), bad); err == nil || len(*reqs) != n {
			t.Fatalf("%q must fail before any request: %v, %d new requests", bad, err, len(*reqs)-n)
		}
	}
}

func TestBaremetalRoutesUseTheUUID(t *testing.T) {
	c, reqs := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"detail":"ok"}`)) })
	ctx := context.Background()
	ref := strings.ToUpper(testBMUUID)
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
	_ = c.FloatingIPs.AssignToBaremetal(ctx, ref, "203.0.113.10")
	want := []string{"update/%s", "%s/power/start_metal", "%s/monitoring", "%s/protection", "%s/move-project", "%s/ssh-keys", "%s/ssh-keys/1", "%s/network", "%s/network", "%s/reinstall", "%s/rescue", "%s/reset-bmc"}
	if len(*reqs) != len(want)+1 {
		t.Fatalf("%d requests", len(*reqs))
	}
	for i, w := range want {
		if p := "/baremetal/" + strings.Replace(w, "%s", testBMUUID, 1); (*reqs)[i].Path != p {
			t.Errorf("request %d: %s, want %s", i, (*reqs)[i].Path, p)
		}
	}
	if p := (*reqs)[len(want)].Path; p != "/floating_ips/assign/baremetal/"+testBMUUID {
		t.Errorf("floating IP assign: %s", p)
	}
	n := len(*reqs)
	for _, bad := range []string{"42", "../vps/1"} {
		if err := c.Baremetal.SetProtection(ctx, bad, true); err == nil || len(*reqs) != n {
			t.Fatalf("%q must fail before any request", bad)
		}
		if err := c.FloatingIPs.AssignToBaremetal(ctx, bad, "203.0.113.10"); err == nil || len(*reqs) != n {
			t.Fatalf("floating IP assign %q must fail before any request", bad)
		}
	}
}

func TestFloatingIPDecodesService(t *testing.T) {
	var ip FloatingIP
	if err := json.Unmarshal([]byte(`{"address":"203.0.113.10","service_type":"baremetal","service_id":"`+testBMUUID+`"}`), &ip); err != nil {
		t.Fatal(err)
	}
	if ip.ServiceType == nil || *ip.ServiceType != "baremetal" || ip.ServiceID == nil || *ip.ServiceID != testBMUUID {
		t.Fatalf("%+v", ip)
	}
}
