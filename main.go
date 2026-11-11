package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"sb_info/sb"
)

// Version is set during build via ldflags
var Version = "dev"

// Endpoints read by the tool, paths relative to /api/. GET only: nothing is
// ever written to the battery.
var v1Endpoints = []string{
	"battery_system",
	"system_data",
	"v1/status",
	"battery",
	"inverter",
	"powermeter",
	"json_api/json_api_configuration",
	"v1/commissioning_assistant/special_function",
}

var v2Endpoints = []string{
	"v2/status",
	"v2/latestdata",
	"v2/configurations",
	"v2/battery",
	"v2/inverter",
	"v2/powermeter",
	"v2/io",
}

// Read-only endpoints the battery's web dashboard uses for cascades (stacks).
// "protect" answers "not a stack" (HTTP 400) on single batteries;
// "inverter/stacking/status" needs a role above User (HTTP 401 for User,
// works for Installer).
var cascadeEndpoints = []string{
	"stacking_status",
	"v1/commissioning_assistant/special_function/stacking/protect",
	"v1/commissioning_assistant/inverter/stacking/status",
}

type Tool struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Member struct {
	Role   string `json:"role"`
	Serial string `json:"serial,omitempty"`
	Status string `json:"status,omitempty"` // from stacking_status, e.g. "WORKING"
}

type Cascade struct {
	Role           string   `json:"role"`
	RoleSource     string   `json:"role_source,omitempty"`
	IsCascade      bool     `json:"is_cascade"`
	StackCapacityW *float64 `json:"stack_capacity_w,omitempty"`
	StackModules   *float64 `json:"stack_modules,omitempty"`
	UnitsByPower   *float64 `json:"units_estimate_by_power,omitempty"`
	UnitsByModules *float64 `json:"units_estimate_by_modules,omitempty"`
	Members        []Member `json:"members,omitempty"`
	MembersSources []string `json:"members_sources,omitempty"`
}

type Report struct {
	Tool        Tool                 `json:"tool"`
	GeneratedAt string               `json:"generated_at"`
	Host        string               `json:"host"`
	User        string               `json:"user"`
	APIs        []string             `json:"apis"`
	Redacted    []string             `json:"redacted"`
	Error       string               `json:"error,omitempty"`
	Cascade     *Cascade             `json:"cascade,omitempty"`
	Endpoints   map[string]sb.Result `json:"endpoints,omitempty"`
}

func main() {
	u := flag.String("u", "User", "username, one of `Vendor`, `Installer`, `Service`, `User`, `Oem`")
	p := flag.String("p", "", "password (or set SB_PASSWORD)")
	h := flag.String("h", "", "host (IP address or host name of the battery)")
	onlyV2 := flag.Bool("2", false, "read only API v2 endpoints (plus the cascade check)")
	noRedact := flag.Bool("no-redact", false, "keep IP and MAC addresses (credentials are always redacted)")
	version := flag.Bool("version", false, "show version information")
	flag.Parse()

	if *version {
		fmt.Printf("sb_info version %s\n", Version)
		os.Exit(0)
	}
	password := *p
	if password == "" {
		password = os.Getenv("SB_PASSWORD")
	}
	if *h == "" || password == "" {
		fmt.Fprintln(os.Stderr, "usage: sb_info -h <host> [-u <user>] [-p <password> | SB_PASSWORD=...] [-2] [-no-redact]")
		os.Exit(2)
	}

	report := Report{
		Tool:        Tool{Name: "sb_info", Version: Version},
		GeneratedAt: time.Now().Format(time.RFC3339),
		Host:        *h,
		User:        *u,
		Redacted:    []string{"credentials"},
	}
	if !*noRedact {
		report.Host = sb.Redacted
		report.Redacted = append(report.Redacted, "network")
	}

	// error messages (e.g. connection errors) contain the host
	scrub := func(msg string) string {
		if *noRedact {
			return msg
		}
		return strings.ReplaceAll(msg, *h, sb.Redacted)
	}

	endpoints := append([]string{}, v2Endpoints...)
	report.APIs = []string{"v2"}
	if !*onlyV2 {
		endpoints = append(append([]string{}, v1Endpoints...), endpoints...)
		report.APIs = []string{"v1", "v2"}
	}
	endpoints = append(endpoints, cascadeEndpoints...)

	sunBat := sb.SunBatInit(*u, password, *h)
	if err := sunBat.Login(); err != nil {
		report.Error = scrub(err.Error())
		write(report)
		os.Exit(1)
	}

	results := map[string]sb.Result{}
	for _, path := range endpoints {
		results[path] = sunBat.Get(path)
		// the battery's web server handles one request at a time: go easy on it
		time.Sleep(200 * time.Millisecond)
	}

	report.Cascade = analyseCascade(results)
	report.Endpoints = map[string]sb.Result{}
	for path, r := range results {
		r.Data = sb.Redact(r.Data, !*noRedact)
		r.Error = scrub(r.Error)
		report.Endpoints[path] = r
	}
	write(report)
}

func write(report Report) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(report); err != nil {
		fmt.Fprintln(os.Stderr, "unable to write report:", err)
		os.Exit(1)
	}
}

// analyseCascade summarises what the battery tells about a cascade: its role,
// the cascade totals of a master (stack_capacity, stack_modules), unit count
// estimates and, if an endpoint lists them, the members with their serials.
func analyseCascade(results map[string]sb.Result) *Cascade {
	c := &Cascade{Role: "unknown"}

	// role: stacking_status (dashboard), else CN_CascadingRole (v1, then v2)
	for _, src := range []struct{ path, key string }{
		{"stacking_status", "role"},
		{"system_data", "CN_CascadingRole"},
		{"v2/configurations", "CN_CascadingRole"},
	} {
		if s, ok := lookup(results[src.path].Data, src.key).(string); ok && s != "" {
			c.Role, c.RoleSource = s, src.path
			break
		}
	}

	bs := results["battery_system"].Data
	stackCapacity, okCap := number(lookup(bs, "stack_capacity"))
	stackModules, okMod := number(lookup(bs, "stack_modules"))
	inverterCapacity, okInv := number(lookup(bs, "battery_system", "system", "inverter_capacity"))
	modules, okModules := number(lookup(bs, "modules"))
	if okCap {
		c.StackCapacityW = &stackCapacity
	}
	if okMod {
		c.StackModules = &stackModules
	}
	if okCap && okInv && stackCapacity > 0 && inverterCapacity > 0 {
		c.UnitsByPower = ratio(stackCapacity, inverterCapacity)
	}
	if okMod && okModules && stackModules > 0 && modules > 0 {
		c.UnitsByModules = ratio(stackModules, modules)
	}

	// members of all cascade endpoints, merged by serial
	bySerial := map[string]int{}
	for _, path := range cascadeEndpoints {
		members := membersFrom(results[path].Data)
		if len(members) == 0 {
			continue
		}
		c.MembersSources = append(c.MembersSources, path)
		for _, m := range members {
			if i, ok := bySerial[m.Serial]; ok && m.Serial != "" {
				if c.Members[i].Status == "" {
					c.Members[i].Status = m.Status
				}
				continue
			}
			if m.Serial != "" {
				bySerial[m.Serial] = len(c.Members)
			}
			c.Members = append(c.Members, m)
		}
	}

	c.IsCascade = (c.Role != "none" && c.Role != "unknown") || (okCap && stackCapacity > 0) || len(c.Members) > 1
	return c
}

// membersFrom extracts the master and slaves of {"master": ..., "slaves": ...},
// also when nested in "data" (like stacking_status). A member group is either
// one object with a "serial" field (stacking/protect), a list of those
// (slaves), or objects keyed by serial, as in stacking_status:
// {"master": {"<serial>": {"ip": ..., "status": "WORKING"}}, "slaves": {...}}.
func membersFrom(data interface{}) []Member {
	obj, ok := data.(map[string]interface{})
	if !ok {
		return nil
	}
	if inner, ok := obj["data"].(map[string]interface{}); ok && obj["master"] == nil {
		obj = inner
	}
	return append(memberGroup("master", obj["master"]), memberGroup("slave", obj["slaves"])...)
}

func memberGroup(role string, v interface{}) []Member {
	var members []Member
	switch g := v.(type) {
	case []interface{}:
		for _, item := range g {
			if m, ok := item.(map[string]interface{}); ok {
				members = append(members, member(role, m))
			}
		}
	case map[string]interface{}:
		if len(g) == 0 {
			return nil
		}
		if _, ok := g["serial"]; ok || !allObjects(g) {
			return []Member{member(role, g)}
		}
		serials := make([]string, 0, len(g))
		for serial := range g {
			serials = append(serials, serial)
		}
		sort.Strings(serials) // stable order: Go maps aren't ordered
		for _, serial := range serials {
			m := g[serial].(map[string]interface{})
			members = append(members, Member{Role: role, Serial: serial, Status: str(m["status"])})
		}
	}
	return members
}

func member(role string, m map[string]interface{}) Member {
	return Member{Role: role, Serial: str(m["serial"]), Status: str(m["status"])}
}

// allObjects: true if all values of g are objects
func allObjects(g map[string]interface{}) bool {
	for _, v := range g {
		if _, ok := v.(map[string]interface{}); !ok {
			return false
		}
	}
	return true
}

func lookup(data interface{}, keys ...string) interface{} {
	for _, k := range keys {
		obj, ok := data.(map[string]interface{})
		if !ok {
			return nil
		}
		data = obj[k]
	}
	return data
}

func number(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(n, 64)
		return f, err == nil
	case float64:
		return n, true
	}
	return 0, false
}

func ratio(a, b float64) *float64 {
	r := math.Round(a/b*100) / 100
	return &r
}

func str(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
