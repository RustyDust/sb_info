package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
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
	IP     string `json:"-"`                // used to read the slaves, never written
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
	Slaves      []UnitReport         `json:"slaves,omitempty"`
}

// UnitReport: what a cascade's slave reports, read via the IP the master lists
type UnitReport struct {
	Serial      string               `json:"serial,omitempty"`
	Host        string               `json:"host"`
	GeneratedAt string               `json:"generated_at"`
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
	noSlaves := flag.Bool("no-slaves", false, "on a cascade master, don't read the slaves")
	out := flag.String("o", "", "report file (UTF-8), default sb_info-<date>-<time>.json; `-` writes to stdout")
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
		fmt.Fprintln(os.Stderr, "usage: sb_info -h <host> [-u <user>] [-p <password> | SB_PASSWORD=...] [-2] [-no-redact] [-no-slaves] [-o <file> | -o -]")
		os.Exit(2)
	}

	now := time.Now()
	target := *out
	if target == "" {
		target = defaultReportName(now)
	}
	report := Report{
		Tool:        Tool{Name: "sb_info", Version: Version},
		GeneratedAt: now.Format(time.RFC3339),
		Host:        *h,
		User:        *u,
		Redacted:    []string{"credentials"},
	}
	if !*noRedact {
		report.Host = sb.Redacted
		report.Redacted = append(report.Redacted, "network")
	}

	endpoints := append([]string{}, v2Endpoints...)
	report.APIs = []string{"v2"}
	if !*onlyV2 {
		endpoints = append(append([]string{}, v1Endpoints...), endpoints...)
		report.APIs = []string{"v1", "v2"}
	}
	endpoints = append(endpoints, cascadeEndpoints...)

	read := func(host string) (map[string]sb.Result, error) {
		return readUnit(*u, password, host, endpoints)
	}
	results, err := read(*h)
	if err != nil {
		report.Error = scrubber(*noRedact, *h)(err.Error())
		save(report, target)
		os.Exit(1)
	}
	report.Cascade = analyseCascade(results)
	report.Endpoints = redactResults(results, *noRedact, scrubber(*noRedact, *h))

	if !*noSlaves && report.Cascade.Role == "master" {
		report.Slaves = readSlaves(report.Cascade, *h, read, *noRedact, time.Now)
	}
	save(report, target)
}

// readUnit logs in once and reads all endpoints of one battery.
func readUnit(user, password, host string, endpoints []string) (map[string]sb.Result, error) {
	sunBat := sb.SunBatInit(user, password, host)
	if err := sunBat.Login(); err != nil {
		return nil, err
	}
	results := map[string]sb.Result{}
	for _, path := range endpoints {
		results[path] = sunBat.Get(path)
		// the battery's web server handles one request at a time: go easy on it
		time.Sleep(200 * time.Millisecond)
	}
	return results, nil
}

// readSlaves reads every slave the master lists with an IP, using the same
// credentials. One login attempt per slave (failed logins make the battery
// block further ones); a failing slave is recorded and the next one is read.
func readSlaves(c *Cascade, masterHost string, read func(string) (map[string]sb.Result, error),
	noRedact bool, now func() time.Time) []UnitReport {
	var reports []UnitReport
	for _, m := range c.Members {
		if m.Role != "slave" || m.IP == "" || m.IP == masterHost {
			continue
		}
		fmt.Fprintf(os.Stderr, "reading slave %s ...\n", m.Serial)
		scrub := scrubber(noRedact, masterHost, m.IP)
		unit := UnitReport{Serial: m.Serial, Host: m.IP, GeneratedAt: now().Format(time.RFC3339)}
		if !noRedact {
			unit.Host = sb.Redacted
		}
		results, err := read(m.IP)
		if err != nil {
			unit.Error = scrub(err.Error())
		} else {
			unit.Cascade = analyseCascade(results)
			unit.Endpoints = redactResults(results, noRedact, scrub)
		}
		reports = append(reports, unit)
	}
	return reports
}

// scrubber removes the given hosts from error messages (e.g. connection
// errors), unless network identifiers are kept.
func scrubber(noRedact bool, hosts ...string) func(string) string {
	return func(msg string) string {
		if noRedact {
			return msg
		}
		for _, h := range hosts {
			if h != "" {
				msg = strings.ReplaceAll(msg, h, sb.Redacted)
			}
		}
		return msg
	}
}

func redactResults(results map[string]sb.Result, noRedact bool, scrub func(string) string) map[string]sb.Result {
	out := make(map[string]sb.Result, len(results))
	for path, r := range results {
		r.Data = sb.Redact(r.Data, !noRedact)
		r.Error = scrub(r.Error)
		out[path] = r
	}
	return out
}

// defaultReportName: sb_info-20261112-094104.json (no colons: valid on Windows)
func defaultReportName(t time.Time) string {
	return "sb_info-" + t.Format("20060102-150405") + ".json"
}

// save writes the report and tells where it went; exits on failure.
func save(report Report, target string) {
	if err := writeReport(report, target); err != nil {
		fmt.Fprintln(os.Stderr, "unable to write report:", err)
		os.Exit(1)
	}
	if target != "-" {
		fmt.Fprintln(os.Stderr, "report written to", target)
	}
}

// writeReport writes the report as UTF-8 JSON to the file target, or to
// stdout for "-". sb_info writes the file itself because shell redirection
// may re-encode the output: Windows PowerShell 5.1's ">" writes UTF-16.
func writeReport(report Report, target string) error {
	if target == "-" {
		return encodeReport(os.Stdout, report)
	}
	f, err := os.Create(target)
	if err != nil {
		return err
	}
	if err := encodeReport(f, report); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func encodeReport(w io.Writer, report Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(report)
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
				if c.Members[i].IP == "" {
					c.Members[i].IP = m.IP
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
			members = append(members, Member{Role: role, Serial: serial, Status: str(m["status"]), IP: str(m["ip"])})
		}
	}
	return members
}

func member(role string, m map[string]interface{}) Member {
	return Member{Role: role, Serial: str(m["serial"]), Status: str(m["status"]), IP: str(m["ip"])}
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
