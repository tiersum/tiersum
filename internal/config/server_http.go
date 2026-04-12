package config

import (
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// HTTPListenAddr returns host:port for net/http Server.Addr.
// Empty or "0.0.0.0" host listens on all interfaces (":port").
func HTTPListenAddr(port int) string {
	if port <= 0 {
		port = 8080
	}
	host := strings.TrimSpace(viper.GetString("server.host"))
	if host == "" || host == "0.0.0.0" {
		return ":" + strconv.Itoa(port)
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

// ServerReadTimeout returns server read/write/idle timeouts from server.timeout (single value).
func ServerReadTimeout() time.Duration {
	d := viper.GetDuration("server.timeout")
	if d <= 0 {
		return 0
	}
	return d
}

// ServerMaxRequestBodyBytes parses server.max_body_size (e.g. 10MB). Zero or invalid uses default.
func ServerMaxRequestBodyBytes() int64 {
	return ParseHumanBytes(viper.GetString("server.max_body_size"), 32<<20)
}
