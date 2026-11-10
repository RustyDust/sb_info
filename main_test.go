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

// values from the cascade master report in issue #152 (2 x sonnenBatterie 10)
func TestAnalyseCascadeMaster(t *testing.T) {
	results := map[string]sb.Result{
		"battery_system": {Status: 200, Data: decode(t, `{"battery_system": {"system": {"inverter_capacity": "4600"}},
			"modules": 2, "stack_capacity": 9200, "stack_modules": 4}`)},
		"system_data":     {Status: 200, Data: decode(t, `{"CN_CascadingRole": "master"}`)},
		"stacking_status": {Status: 200, Data: decode(t, `{"role": "master", "data": null, "country": "DE"}`)},
		"v1/commissioning_assistant/special_function/stacking/protect": {Status: 200, Data: decode(t,
			`{"master": {"serial": "100001", "protect_active": true}, "slaves": [{"serial": 170167}]}`)},
	}
	c := analyseCascade(results)
	if !c.IsCascade || c.Role != "master" || c.RoleSource != "stacking_status" {
		t.Errorf("role: got %+v", c)
	}
	if *c.StackCapacityW != 9200 || *c.StackModules != 4 || *c.UnitsByPower != 2 || *c.UnitsByModules != 2 {
		t.Errorf("totals: got %v %v %v %v", *c.StackCapacityW, *c.StackModules, *c.UnitsByPower, *c.UnitsByModules)
	}
	want := []Member{{"master", "100001"}, {"slave", "170167"}}
	if len(c.Members) != 2 || c.Members[0] != want[0] || c.Members[1] != want[1] {
		t.Errorf("members: got %+v", c.Members)
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
