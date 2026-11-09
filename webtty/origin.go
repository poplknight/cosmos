package webtty

import "net/url"

func defaultOrigin(origin, host string) bool {
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	if parsed.Host == host {
		return true
	}
	name := parsed.Hostname()
	return name == "localhost" || name == "127.0.0.1" || name == "::1"
}
