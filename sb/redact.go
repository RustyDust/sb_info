package sb

import "regexp"

const Redacted = "<redacted>"

// Credentials are never needed to analyse a report: always redacted
// (e.g. IN_LocalAPIToken in json_api_configuration).
var secretKey = regexp.MustCompile(`(?i)token|password|passwd|secret`)

// Network identifiers: redacted unless explicitly requested, since reports
// often get posted publicly (public WAN IP, VPN IP, MAC address).
var networkKeys = map[string]bool{
	"lanip":       true,
	"vpnip":       true,
	"wanip":       true,
	"wlanip":      true,
	"mac_address": true,
}

// Redact returns a copy of v with secret values (and, if network is true,
// network identifiers) replaced by "<redacted>". Empty values stay empty.
func Redact(v interface{}, network bool) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(val))
		for k, item := range val {
			if isScalar(item) && !isEmpty(item) && (secretKey.MatchString(k) || (network && networkKeys[k])) {
				out[k] = Redacted
			} else {
				out[k] = Redact(item, network)
			}
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(val))
		for i, item := range val {
			out[i] = Redact(item, network)
		}
		return out
	default:
		return v
	}
}

func isScalar(v interface{}) bool {
	switch v.(type) {
	case map[string]interface{}, []interface{}:
		return false
	}
	return true
}

func isEmpty(v interface{}) bool {
	return v == nil || v == ""
}
