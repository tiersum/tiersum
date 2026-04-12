package svcimpl

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/tiersum/tiersum/pkg/types"
)

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func randomHex(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func newBrowserAccessToken() (plaintext string, hashHex string, err error) {
	suffix, err := randomHex(32)
	if err != nil {
		return "", "", err
	}
	plain := "ts_u_" + suffix
	return plain, sha256Hex(plain), nil
}

func newSessionCookieValue() (plaintext string, hashHex string, err error) {
	suffix, err := randomHex(32)
	if err != nil {
		return "", "", err
	}
	// Opaque cookie value (no prefix); only hash is stored server-side.
	return suffix, sha256Hex(suffix), nil
}

func newAPIKeyPlaintext(scope string) (plaintext string, hashHex string, err error) {
	suffix, err := randomHex(24)
	if err != nil {
		return "", "", err
	}
	var prefix string
	switch scope {
	case types.AuthScopeAdmin:
		prefix = "tsk_admin_"
	default:
		prefix = "tsk_live_"
	}
	plain := prefix + suffix
	return plain, sha256Hex(plain), nil
}

func apiKeyScopeRank(scope string) int {
	switch scope {
	case types.AuthScopeRead:
		return 1
	case types.AuthScopeWrite:
		return 2
	case types.AuthScopeAdmin:
		return 3
	default:
		return 0
	}
}

func normalizeUserAgent(ua string) string {
	if len(ua) > 512 {
		ua = ua[:512]
	}
	// ASCII lowercase for stable comparison
	b := []byte(ua)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

func ipPrefixForBind(clientIP string) string {
	ip := trimHost(clientIP)
	if ip == "" {
		return ""
	}
	// IPv4: keep first three octets
	for i := 0; i < len(ip); i++ {
		if ip[i] == '.' {
			c := 0
			for j := 0; j < len(ip); j++ {
				if ip[j] == '.' {
					c++
					if c == 3 {
						return ip[:j]
					}
				}
			}
			return ip
		}
		if ip[i] == ':' {
			// IPv6: first four hextet groups (very coarse /64-ish prefix)
			parts := 0
			for j := 0; j <= len(ip); j++ {
				if j == len(ip) || ip[j] == ':' {
					parts++
					if parts == 4 {
						return ip[:j]
					}
				}
			}
			return ip
		}
	}
	return ip
}

func trimHost(host string) string {
	if host == "" {
		return ""
	}
	// Strip bracketed IPv6 [::1]:port
	if host[0] == '[' {
		if i := findByte(host, ']'); i > 0 {
			return host[1:i]
		}
	}
	// Strip :port
	if i := findByte(host, ':'); i > 0 {
		// If more than one colon, likely IPv6 without brackets — do not strip
		if countByte(host, ':') == 1 {
			return host[:i]
		}
	}
	return host
}

func findByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func countByte(s string, b byte) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			n++
		}
	}
	return n
}

func fingerprintStrictHash(ua, ipPrefix, tz, clientSignal string) string {
	return sha256Hex(fmt.Sprintf("%s|%s|%s|%s", normalizeUserAgent(ua), ipPrefix, tz, clientSignal))
}

func sessionRequestLooksConsistent(sessionIPPrefix, sessionUANorm, remoteIP, userAgent string) bool {
	curIP := ipPrefixForBind(remoteIP)
	curUA := normalizeUserAgent(userAgent)
	if sessionIPPrefix != "" && curIP != "" && sessionIPPrefix != curIP {
		return false
	}
	if sessionUANorm == "" {
		return true
	}
	if curUA == sessionUANorm {
		return true
	}
	// Allow minor UA upgrades: same leading prefix
	const prefixLen = 120
	sa := sessionUANorm
	ca := curUA
	if len(sa) > prefixLen {
		sa = sa[:prefixLen]
	}
	if len(ca) > prefixLen {
		ca = ca[:prefixLen]
	}
	return sa == ca || (len(ca) >= len(sessionUANorm) && ca[:len(sessionUANorm)] == sessionUANorm) ||
		(len(sessionUANorm) >= len(ca) && sa[:len(ca)] == ca)
}
