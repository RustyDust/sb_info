# sb_info

Reads everything the local API of a SonnenBatterie reveals (API v1 and v2,
plus the cascade information the battery's own web dashboard uses) and writes
it as one JSON report. Useful for bug reports and for analysing installations
the integration doesn't handle yet.

The tool only reads (HTTP GET); it never changes anything on the battery.
A copy is maintained in the `tools/sb_info` folder of
[ha_sonnenbatterie](https://github.com/weltmeyer/ha_sonnenbatterie), the Home Assistant integration.

## Usage

Download the version for your operating system from the [Releases](https://github.com/RustyDust/sb_info/releases) page, unpack it and run it in a terminal:

```bash
# password as parameter ...
./sb_info -h 192.168.0.22 -p <password> > report.json
# ... or from the environment (keeps it out of the shell history)
SB_PASSWORD=<password> ./sb_info -h 192.168.0.22 > report.json
```

On Windows use `sb_info.exe`; set the password with `$env:SB_PASSWORD="..."` (PowerShell).

| Parameter | Meaning |
|---|---|
| `-h <host>` | IP address or host name of the battery (required) |
| `-u <user>` | user type: `User` (default), `Installer`, `Service`, `Vendor`, `Oem` |
| `-p <password>` | password; alternatively the environment variable `SB_PASSWORD` |
| `-2` | read only the API v2 endpoints (the cascade check always runs) |
| `-no-redact` | keep IP and MAC addresses in the report |
| `-version` | show the version |

Exit codes: `0` report written (individual endpoints may still have failed,
see their `status`/`error`), `1` login failed or battery not reachable (the
report then only contains `error`), `2` wrong usage.

## Privacy

The report is meant to be shared, e.g. in a GitHub issue:

- The password is never written to the report.
- Credentials are always redacted, e.g. the API token (`IN_LocalAPIToken`).
- IP addresses (LAN, WAN, VPN) and the MAC address are redacted unless
  `-no-redact` is given, including the host in error messages.
- The serial number is kept: it's needed to analyse cascades.

## Report format

```json
{
  "tool": {"name": "sb_info", "version": "0.4.0"},
  "generated_at": "2026-11-10T21:47:47+01:00",
  "host": "<redacted>",
  "user": "User",
  "apis": ["v1", "v2"],
  "redacted": ["credentials", "network"],
  "cascade": {
    "role": "master",
    "role_source": "stacking_status",
    "is_cascade": true,
    "stack_capacity_w": 9200,
    "stack_modules": 4,
    "units_estimate_by_power": 2,
    "units_estimate_by_modules": 2,
    "members": [{"role": "master", "serial": "170166"}, {"role": "slave", "serial": "..."}],
    "members_source": "v1/commissioning_assistant/special_function/stacking/protect"
  },
  "endpoints": {
    "battery_system": {"status": 200, "data": {"...": "..."}},
    "v1/commissioning_assistant/special_function/stacking/protect": {"status": 400, "error": "not a stack"}
  }
}
```

- `endpoints`: one entry per API path (relative to `/api/`) with the HTTP
  `status` and either `data` (the battery's JSON, numbers exactly as sent) or
  `error`. A failing endpoint doesn't stop the report.
- `cascade`: a summary of what the battery tells about a cascade (several
  batteries acting as one system):
  - `role`: `none`, `master` or `slave`, from `stacking_status`, else
    `CN_CascadingRole` (`system_data`, then `v2/configurations`)
  - `stack_capacity_w` / `stack_modules`: the whole cascade's inverter power
    and module count, reported by a master (`0` on single batteries)
  - `units_estimate_by_power` / `..._by_modules`: number of units, estimated as
    `stack_capacity / inverter_capacity` and `stack_modules / modules`
    (assumes identical units; a fraction hints at mixed units)
  - `members`: the cascade's units with their serials, if one of the
    dashboard's cascade endpoints lists them (`members_source`)

### Endpoints

| API | Paths |
|---|---|
| v1 | `battery_system`, `system_data`, `v1/status`, `battery`, `inverter`, `powermeter`, `json_api/json_api_configuration`, `v1/commissioning_assistant/special_function` |
| v2 | `v2/status`, `v2/latestdata`, `v2/configurations`, `v2/battery`, `v2/inverter`, `v2/powermeter`, `v2/io` |
| cascade | `stacking_status`, `v1/commissioning_assistant/special_function/stacking/protect` (`400 "not a stack"` on single batteries), `v1/commissioning_assistant/inverter/stacking/status` (needs a role above `User`) |

### Examples with `jq`

```bash
jq .cascade report.json                                   # cascade summary
jq '.endpoints | map_values(.status)' report.json         # status of every endpoint
jq '.endpoints["v2/inverter"].data | with_entries(select(.key | test("^[ipu]pv")))' report.json   # PV values
```

## Building from source

Requires Go 1.25 or later and GNU tar (`gtar` on macOS: `brew install gnu-tar`).
The version is taken from the latest git tag (override with `VERSION=...`).

```bash
make test       # go vet + unit tests
make macarm     # or: mactel, amd64, arm64, wintel, winarm
make all        # all platforms
```

Binaries are created in `bin/<os>/<arch>/sb_info[.exe]` and packed as
`bin/sb_info-<version>-<os>-<arch>.tar.gz`. Creating a GitHub release builds and attaches
the binaries for all platforms automatically (after running the tests).

```bash
go run . -h 192.168.0.22 -p <password>    # run without building
```
