package sb

import (
	"encoding/json"
	"testing"
)

const sample = `{
	"IN_LocalAPIReadActive": "1",
	"IN_LocalAPIToken": "cd46d0f2-0000-0000-0000-000000000000",
	"battery_system": {"network": {"lanip": "192.168.1.2", "wanip": "203.0.113.7", "wlanip": ""},
	                   "system": {"mac_address": "00:e0:4b:00:00:01", "serial": "100000"}},
	"meters": [{"password": "x", "w_total": 1}],
	"slaves": {"100002": {"ip": "192.168.1.3", "status": "WORKING"}}
}`

func TestRedact(t *testing.T) {
	var data interface{}
	if err := json.Unmarshal([]byte(sample), &data); err != nil {
		t.Fatal(err)
	}

	check := func(network bool, path []string, want string) {
		t.Helper()
		v := Redact(data, network)
		for _, k := range path {
			switch x := v.(type) {
			case map[string]interface{}:
				v = x[k]
			case []interface{}:
				v = x[0]
			}
		}
		if v != want {
			t.Errorf("network=%v %v: got %v, want %v", network, path, v, want)
		}
	}

	for _, network := range []bool{true, false} {
		// credentials: always
		check(network, []string{"IN_LocalAPIToken"}, Redacted)
		check(network, []string{"meters", "0", "password"}, Redacted)
		// unrelated values: never
		check(network, []string{"IN_LocalAPIReadActive"}, "1")
		check(network, []string{"battery_system", "system", "serial"}, "100000")
		// empty values stay empty
		check(network, []string{"battery_system", "network", "wlanip"}, "")
	}
	// network identifiers: only when requested
	check(true, []string{"battery_system", "network", "wanip"}, Redacted)
	check(true, []string{"battery_system", "system", "mac_address"}, Redacted)
	check(false, []string{"battery_system", "network", "wanip"}, "203.0.113.7")
	check(false, []string{"battery_system", "system", "mac_address"}, "00:e0:4b:00:00:01")
	// cascade members (stacking_status)
	check(true, []string{"slaves", "100002", "ip"}, Redacted)
	check(false, []string{"slaves", "100002", "ip"}, "192.168.1.3")
	check(true, []string{"slaves", "100002", "status"}, "WORKING")
}
