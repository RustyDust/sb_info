package main

import (
	"encoding/json"
	"strings"
	"testing"

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
	checkMembers(t, c.Members, []Member{{"master", "100001", "WORKING"}, {"slave", "100002", "WORKING"}})
	if len(c.MembersSources) != 2 || c.MembersSources[0] != "stacking_status" {
		t.Errorf("sources: got %v", c.MembersSources)
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
	checkMembers(t, c.Members, []Member{
		{"master", "200001", "WORKING"}, {"slave", "200002", "WORKING"}, {"slave", "200003", "WORKING"}})
	if len(c.MembersSources) != 3 {
		t.Errorf("sources: got %v", c.MembersSources)
	}
}

func checkMembers(t *testing.T, got, want []Member) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("members: got %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
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
	checkMembers(t, m, []Member{{"master", "1", "WORKING"}, {"slave", "2", "ERROR"}, {"slave", "3", ""}})
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
	checkMembers(t, c.Members, []Member{{"master", "1", "WORKING"}, {"slave", "2", "OK"}, {"slave", "3", ""}, {"slave", "", ""}})
}
