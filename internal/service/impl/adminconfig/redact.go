package adminconfig

import (
	"fmt"
	"regexp"
	"strings"
)

const redactedPlaceholder = "[redacted]"

// RedactConfigMap returns a deep copy of m with sensitive keys and credential-like string values masked.
func RedactConfigMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	return redactMap(m)
}

func redactMap(m map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, val := range m {
		if configKeySensitive(k) {
			out[k] = redactedScalar(val)
			continue
		}
		out[k] = redactValue(val)
	}
	return out
}

func redactValue(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		return redactMap(t)
	case []interface{}:
		out := make([]interface{}, len(t))
		for i, e := range t {
			out[i] = redactValue(e)
		}
		return out
	case string:
		return redactStringLeaf(t)
	default:
		return v
	}
}

func configKeyAllowlist(lowerKey string) bool {
	switch lowerKey {
	case "max_tokens", "min_tokens", "temperature", "top_p", "sample_ratio",
		"progressive_answer_max_tokens", "progressive_answer_max_references",
		"progressive_answer_excerpt_max_bytes", "force_sample_query_param", "force_sample_header",
		"token_expiry_mode", "disable_think", "num_cpu", "gomaxprocs", "compiler",
		"span_count", "max_size", "max_chunk_size", "chapter_max_tokens", "sliding_stride_tokens",
		"branch_recall_multiplier", "branch_recall_floor", "branch_recall_ceiling",
		"hnsw_m", "hnsw_ef_search", "vector_dim", "cold_doc_count":
		return true
	default:
		return false
	}
}

func configKeySensitive(key string) bool {
	lk := strings.ToLower(strings.TrimSpace(key))
	if lk == "" {
		return false
	}
	if configKeyAllowlist(lk) {
		return false
	}
	switch lk {
	case "password", "passwd", "pwd", "passphrase",
		"api_key", "apikey", "apisecret", "apisecretkey",
		"secret", "client_secret", "clientsecret",
		"private_key", "privatekey", "privkey",
		"access_token", "refresh_token", "id_token", "session_token",
		"authorization", "cookie", "set_cookie", "set-cookie",
		"dsn", "database_url", "databaseurl", "connection_string", "connectionstring",
		"jdbc_url", "jdbcurl", "conn_string",
		"webhook_secret", "signing_secret", "signing_key", "encryption_key", "encrypted_password",
		"otp_secret", "totp_secret", "mfa_secret", "ssh_key", "tls_key", "tlskey":
		return true
	}
	suffixes := []string{
		"_password", "_passwd", "_pwd", "_passphrase",
		"_secret", "_api_key", "_apikey", "_private_key", "_client_secret",
		"_access_token", "_refresh_token", "_id_token", "_session_token",
		"_dsn", "_webhook_secret", "_signing_secret", "_signing_key", "_encryption_key",
		"_otp_secret", "_totp_secret", "_mfa_secret",
	}
	for _, suf := range suffixes {
		if strings.HasSuffix(lk, suf) {
			return true
		}
	}
	if strings.HasSuffix(lk, "_bearer") || strings.HasSuffix(lk, "_credential") || strings.HasSuffix(lk, "_credentials") {
		return true
	}
	if strings.HasSuffix(lk, "_token") && !strings.HasSuffix(lk, "_tokens") && !strings.HasSuffix(lk, "max_tokens") {
		return true
	}
	if strings.HasPrefix(lk, "secret_") {
		return true
	}
	if strings.HasSuffix(lk, "_key") {
		if strings.Contains(lk, "api") || strings.Contains(lk, "secret") || strings.Contains(lk, "private") ||
			strings.Contains(lk, "encrypt") || strings.Contains(lk, "signing") || strings.Contains(lk, "webhook") ||
			strings.Contains(lk, "license") {
			return true
		}
	}
	return false
}

func redactedScalar(v interface{}) string {
	if v == nil {
		return redactedPlaceholder
	}
	switch t := v.(type) {
	case string:
		if strings.TrimSpace(t) == "" {
			return redactedPlaceholder
		}
		return fmt.Sprintf("%s (scalar was %d chars)", redactedPlaceholder, len(t))
	default:
		return redactedPlaceholder
	}
}

func redactStringLeaf(s string) string {
	if s == "" {
		return s
	}
	t := strings.TrimSpace(s)
	if strings.Contains(t, "BEGIN") && strings.Contains(t, "PRIVATE") {
		return redactedPlaceholder + ":pem-private-block"
	}
	if looksLikeJWT(t) {
		return redactedPlaceholder + ":jwt"
	}
	if looksLikePrefixedSecret(t) {
		return redactedPlaceholder + ":credential-prefix"
	}
	if looksLikeAWSAccessKeyID(t) {
		return redactedPlaceholder + ":aws-access-key-id"
	}
	if redacted := redactURLUserinfo(t); redacted != t {
		return redacted
	}
	return s
}

func looksLikeJWT(s string) bool {
	if strings.Count(s, ".") != 2 {
		return false
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 || len(s) < 40 {
		return false
	}
	for _, p := range parts {
		if len(p) < 10 || !isLikelyBase64URLSegment(p) {
			return false
		}
	}
	return true
}

func isLikelyBase64URLSegment(p string) bool {
	dash := 0
	underscore := 0
	alnum := 0
	for _, r := range p {
		switch {
		case r == '-' || r == '_' || r == '=':
			if r == '-' {
				dash++
			} else if r == '_' {
				underscore++
			}
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
			alnum++
		default:
			return false
		}
	}
	if alnum < len(p)/2 {
		return false
	}
	if underscore == 0 && dash == 0 && alnum == len(p) {
		allNum := true
		for _, r := range p {
			if r < '0' || r > '9' {
				allNum = false
				break
			}
		}
		if allNum {
			return false
		}
	}
	return true
}

func looksLikePrefixedSecret(s string) bool {
	ls := strings.ToLower(s)
	prefixes := []string{
		"sk-", "sk_live", "sk_test", "sk_proj",
		"pk_live", "pk_test",
		"rk_live", "rk_test",
		"ts_u_", "tsk_live", "tsk_admin",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(ls, p) {
			return len(s) >= 12
		}
	}
	return false
}

func looksLikeAWSAccessKeyID(s string) bool {
	if len(s) != 20 {
		return false
	}
	p := s[:4]
	if p != "AKIA" && p != "ASIA" {
		return false
	}
	for _, r := range s[4:] {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

var urlUserinfoPattern = regexp.MustCompile(`(?i)([a-z][a-z0-9+.-]*://)([^/?#:]+):([^@/?#]*)(@)`)

func redactURLUserinfo(s string) string {
	if !strings.Contains(s, "://") || !strings.Contains(s, "@") {
		return s
	}
	return urlUserinfoPattern.ReplaceAllString(s, "${1}*:*${4}")
}

