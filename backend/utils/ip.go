package utils

import (
	"net"
	"net/http"
)

func RealClientIP(r *http.Request) string {
	if xfwd := r.Header.Get("X-Forwarded-For"); xfwd != "" {
		return xfwd
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
