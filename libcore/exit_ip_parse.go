package libcore

import (
	"encoding/json"
	"errors"
	"net/netip"
)

func parseExitIP(body string) (string, error) {
	var info struct {
		IP          string `json:"ip"`
		CountryCode string `json:"countryCode"`
	}
	if json.Unmarshal([]byte(body), &info) != nil {
		return "", errors.New("exit_ip_invalid_response")
	}
	ip, err := netip.ParseAddr(info.IP)
	if err != nil || !ip.IsGlobalUnicast() || ip.IsPrivate() {
		return "", errors.New("exit_ip_invalid_response")
	}
	cc := info.CountryCode
	if len(cc) != 2 || cc[0] < 'A' || cc[0] > 'Z' || cc[1] < 'A' || cc[1] > 'Z' {
		return "", errors.New("exit_ip_invalid_response")
	}
	return ip.String() + "\n" + cc, nil
}
