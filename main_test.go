package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sb_info/sb"
)

func decode(t *testing.T, s string) interface{} {
	t.Helper()
	var v interface{}
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

// values from the cascade master report in issue #152 (2 x sonnenBatterie 10,
// user "User"; serials and IPs replaced)
func TestAnalyseCascadeMaster(t *testing.T) {
	results := map[string]sb.Result{
		"battery_system": {Status: 200, Data: decode(t, `{"battery_system": {"system": {"inverter_capacity": "4600"}},
			"modules": 2, "stack_capacity": 9200, "stack_modules": 4}`)},
		"system_data": {Status: 200, Data: decode(t, `{"CN_CascadingRole": "master"}`)},
		"stacking_status": {Status: 200, Data: decode(t, `{"country": "BE", "role": "master", "data": {
			"master": {"100001": {"ip": "192.0.2.51", "status": "WORKING"}},
			"slaves": {"100002": {"ip": "192.0.2.114", "status": "WORKING"}}}}`)},
		"v1/commissioning_assistant/special_function/stacking/protect": {Status: 200, Data: decode(t,
			`{"master": {"backup_buffer": 0, "protect_active": false, "serial": "100001"},
			"slaves": [{"backup_buffer": 0, "protect_active": false, "serial": "100002"}]}`)},
		"v1/commissioning_assistant/inverter/stacking/status": {Status: 401, Error: "401 Unauthorized"},
	}
	c := analyseCascade(results)
	if !c.IsCascade || c.Role != "master" || c.RoleSource != "stacking_status" {
		t.Errorf("role: got %+v", c)
	}
	if *c.StackCapacityW != 9200 || *c.StackModules != 4 || *c.UnitsByPower != 2 || *c.UnitsByModules != 2 {
		t.Errorf("totals: got %v %v %v %v", *c.StackCapacityW, *c.StackModules, *c.UnitsByPower, *c.UnitsByModules)
	}
	checkMembers(t, c.Members, []wantMember{{"master", "100001", "WORKING"}, {"slave", "100002", "WORKING"}})
	if len(c.MembersSources) != 2 || c.MembersSources[0] != "stacking_status" {
		t.Errorf("sources: got %v", c.MembersSources)
	}
	if c.Members[0].IP != "192.0.2.51" || c.Members[1].IP != "192.0.2.114" {
		t.Errorf("IPs: got %+v", c.Members)
	}
	if b, _ := json.Marshal(c); strings.Contains(string(b), "192.0.2") {
		t.Errorf("IPs must not be written: %s", b)
	}
}

// three-unit cascade master read as Installer: all three endpoints list the
// members (serials and IPs replaced)
func TestAnalyseCascadeMasterInstaller(t *testing.T) {
	inverter := `"grid_code_status": "set", "installed_version1": "3.18.0", "installed_version2": "4.7.14", "inverter_grid_code": "4901"`
	results := map[string]sb.Result{
		"battery_system": {Status: 200, Data: decode(t, `{"battery_system": {"system": {"inverter_capacity": "4600"}},
			"modules": 4, "stack_capacity": 13800, "stack_modules": 12}`)},
		"stacking_status": {Status: 200, Data: decode(t, `{"country": "DE", "role": "master", "data": {
			"master": {"200001": {"ip": "192.0.2.28", "status": "WORKING"}},
			"slaves": {"200003": {"ip": "192.0.2.66", "status": "WORKING"},
			           "200002": {"ip": "192.0.2.39", "status": "WORKING"}}}}`)},
		"v1/commissioning_assistant/special_function/stacking/protect": {Status: 200, Data: decode(t,
			`{"master": {"backup_buffer": 0, "protect_active": false, "serial": "200001"},
			"slaves": [{"backup_buffer": 0, "protect_active": false, "serial": "200002"},
			           {"backup_buffer": 0, "protect_active": false, "serial": "200003"}]}`)},
		"v1/commissioning_assistant/inverter/stacking/status": {Status: 200, Data: decode(t,
			`{"master": {`+inverter+`, "serial": "200001"},
			"slaves": [{`+inverter+`, "ip": "192.0.2.39", "serial": "200002"},
			           {`+inverter+`, "ip": "192.0.2.66", "serial": "200003"}]}`)},
	}
	c := analyseCascade(results)
	if !c.IsCascade || c.Role != "master" || *c.UnitsByPower != 3 || *c.UnitsByModules != 3 {
		t.Errorf("got %+v", c)
	}
	checkMembers(t, c.Members, []wantMember{
		{"master", "200001", "WORKING"}, {"slave", "200002", "WORKING"}, {"slave", "200003", "WORKING"}})
	if len(c.MembersSources) != 3 {
		t.Errorf("sources: got %v", c.MembersSources)
	}
}

// role, serial and status of a member (IPs are checked separately)
type wantMember struct{ Role, Serial, Status string }

func checkMembers(t *testing.T, got []Member, want []wantMember) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("members: got %+v, want %+v", got, want)
	}
	for i := range want {
		if (wantMember{got[i].Role, got[i].Serial, got[i].Status}) != want[i] {
			t.Errorf("members: got %+v, want %+v", got, want)
			return
		}
	}
}

// values from a single battery (sonnenBatterie 10 performance)
func TestAnalyseCascadeSingle(t *testing.T) {
	results := map[string]sb.Result{
		"battery_system": {Status: 200, Data: decode(t, `{"battery_system": {"system": {"inverter_capacity": "8000"}},
			"modules": 4, "stack_capacity": 0, "stack_modules": 0}`)},
		"stacking_status": {Status: 200, Data: decode(t, `{"role": "none", "data": null, "country": "DE"}`)},
		"v1/commissioning_assistant/special_function/stacking/protect": {Status: 400, Error: "not a stack"},
		"v1/commissioning_assistant/inverter/stacking/status":          {Status: 401, Error: "401 Unauthorized"},
	}
	c := analyseCascade(results)
	if c.IsCascade || c.Role != "none" || c.UnitsByPower != nil || len(c.Members) != 0 {
		t.Errorf("got %+v", c)
	}
}

// -2 mode: no battery_system, role from v2/configurations
func TestAnalyseCascadeV2Only(t *testing.T) {
	results := map[string]sb.Result{
		"v2/configurations": {Status: 200, Data: decode(t, `{"CN_CascadingRole": "slave"}`)},
	}
	c := analyseCascade(results)
	if !c.IsCascade || c.Role != "slave" || c.RoleSource != "v2/configurations" || c.StackCapacityW != nil {
		t.Errorf("got %+v", c)
	}
}

func TestMembersNestedInData(t *testing.T) {
	m := membersFrom(decode(t, `{"role": "master", "data": {"master": {"serial": "1"}, "slaves": [{"serial": "2"}, {"serial": "3"}]}}`))
	if len(m) != 3 || m[2].Serial != "3" {
		t.Errorf("got %+v", m)
	}
}

func TestMembersKeyedBySerial(t *testing.T) {
	m := membersFrom(decode(t, `{"role": "master", "data": {"master": {"1": {"status": "WORKING"}},
		"slaves": {"3": {}, "2": {"status": "ERROR"}}}}`))
	checkMembers(t, m, []wantMember{{"master", "1", "WORKING"}, {"slave", "2", "ERROR"}, {"slave", "3", ""}})
	if m := membersFrom(decode(t, `{"role": "master", "data": {"master": {}, "slaves": {}}}`)); len(m) != 0 {
		t.Errorf("empty groups: got %+v", m)
	}
	if m := membersFrom(decode(t, `{"master": {"name": "no serial"}}`)); len(m) != 1 || m[0].Serial != "" {
		t.Errorf("no serial: got %+v", m)
	}
}

func TestMembersMergedBySerial(t *testing.T) {
	c := analyseCascade(map[string]sb.Result{
		"stacking_status": {Status: 200, Data: decode(t, `{"role": "master", "data": {
			"master": {"1": {"status": "WORKING"}}, "slaves": {"2": {}}}}`)},
		"v1/commissioning_assistant/special_function/stacking/protect": {Status: 200, Data: decode(t,
			`{"master": {"serial": 1, "status": "other"}, "slaves": [{"serial": "2", "status": "OK"}, {"serial": "3"}, {}]}`)},
	})
	// first endpoint wins, missing status is filled in, members without serial are kept
	checkMembers(t, c.Members, []wantMember{{"master", "1", "WORKING"}, {"slave", "2", "OK"}, {"slave", "3", ""}, {"slave", "", ""}})
}

func TestDefaultReportName(t *testing.T) {
	got := defaultReportName(time.Date(2026, 11, 12, 9, 41, 4, 0, time.Local))
	if got != "sb_info-20261112-094104.json" {
		t.Errorf("got %q", got)
	}
}

// the report file is plain UTF-8 JSON without BOM, also for non-ASCII text
func TestWriteReportUTF8(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	report := Report{Tool: Tool{Name: "sb_info"}, Host: "<redacted>", Error: "Verbindung abgelehnt: Gerät"}
	if err := writeReport(report, path); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.HasPrefix(b, []byte{0xef, 0xbb, 0xbf}) || bytes.HasPrefix(b, []byte{0xff, 0xfe}) || b[0] != '{' {
		t.Errorf("unexpected start: % x", b[:4])
	}
	if !bytes.Contains(b, []byte(`"host": "<redacted>"`)) || !bytes.Contains(b, []byte("Gerät")) {
		t.Errorf("content: %s", b)
	}
	var back Report
	if err := json.Unmarshal(b, &back); err != nil || back.Error != report.Error {
		t.Errorf("round trip: %v %+v", err, back)
	}
}

func TestWriteReportBadPath(t *testing.T) {
	if err := writeReport(Report{}, filepath.Join(t.TempDir(), "missing", "report.json")); err == nil {
		t.Error("expected an error for a missing directory")
	}
}

// fake reader for readSlaves: canned results per host, counting logins
type fakeUnits struct {
	results map[string]map[string]sb.Result
	errors  map[string]error
	reads   []string
}

func (f *fakeUnits) read(host string) (map[string]sb.Result, error) {
	f.reads = append(f.reads, host)
	if err := f.errors[host]; err != nil {
		return nil, err
	}
	return f.results[host], nil
}

func slaveResults(t *testing.T) map[string]sb.Result {
	return map[string]sb.Result{
		"stacking_status": {Status: 200, Data: decode(t, `{"country": "DE", "data": null, "role": "slave"}`)},
		"v1/commissioning_assistant/special_function/stacking/protect": {Status: 400, Error: "not a stack"},
		"v2/status": {Status: 200, Data: decode(t, `{"USOC": 85, "Pac_total_W": 602}`)},
		"battery_system": {Status: 200, Data: decode(t,
			`{"battery_system": {"network": {"lanip": "192.0.2.39"}}, "modules": 4, "stack_capacity": 0}`)},
	}
}

func TestReadSlaves(t *testing.T) {
	c := &Cascade{Role: "master", Members: []Member{
		{Role: "master", Serial: "200001", IP: "192.0.2.28"},
		{Role: "slave", Serial: "200002", IP: "192.0.2.39"},
		{Role: "slave", Serial: "200003", IP: "192.0.2.66"},
		{Role: "slave", Serial: "200004"}, // no IP: can't be read
	}}
	f := &fakeUnits{
		results: map[string]map[string]sb.Result{"192.0.2.39": slaveResults(t)},
		errors:  map[string]error{"192.0.2.66": fmt.Errorf("Post \"http://192.0.2.66/api/session\": 401 Unauthorized")},
	}
	now := func() time.Time { return time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC) }
	units := readSlaves(c, "192.0.2.28", f.read, false, now)

	// the master and the slave without IP aren't read; one read (login) per slave
	if len(f.reads) != 2 || f.reads[0] != "192.0.2.39" || f.reads[1] != "192.0.2.66" {
		t.Fatalf("reads: got %v", f.reads)
	}
	if len(units) != 2 {
		t.Fatalf("units: got %+v", units)
	}
	ok, failed := units[0], units[1]
	if ok.Serial != "200002" || ok.Host != sb.Redacted || ok.Error != "" || ok.Cascade.Role != "slave" ||
		ok.GeneratedAt != "2026-11-12T09:00:00Z" {
		t.Errorf("slave: got %+v", ok)
	}
	if ok.Endpoints["v2/status"].Status != 200 {
		t.Errorf("endpoints: got %+v", ok.Endpoints)
	}
	// network identifiers redacted, also inside the slave's data and errors
	b, _ := json.Marshal(units)
	if strings.Contains(string(b), "192.0.2") {
		t.Errorf("IPs must be redacted: %s", b)
	}
	if failed.Serial != "200003" || failed.Endpoints != nil || !strings.Contains(failed.Error, "401") {
		t.Errorf("failed slave: got %+v", failed)
	}

	// -no-redact keeps the IPs
	f.reads = nil
	units = readSlaves(c, "192.0.2.28", f.read, true, now)
	if units[0].Host != "192.0.2.39" || !strings.Contains(units[1].Error, "192.0.2.66") {
		t.Errorf("no-redact: got %+v", units)
	}
}

func TestReadSlavesNothingToRead(t *testing.T) {
	f := &fakeUnits{}
	c := &Cascade{Role: "master", Members: []Member{{Role: "master", Serial: "1", IP: "192.0.2.1"}}}
	if units := readSlaves(c, "192.0.2.1", f.read, false, time.Now); units != nil || len(f.reads) != 0 {
		t.Errorf("got %+v, reads %v", units, f.reads)
	}
}
