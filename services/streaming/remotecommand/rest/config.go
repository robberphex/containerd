package rest

import (
	"net/http"
	"net/url"
)

// Config holds the common attributes that can be passed to a Kubernetes client on
// initialization.
type Config struct {

	// Proxy is the proxy func to be used for all requests made by this
	// transport. If Proxy is nil, http.ProxyFromEnvironment is used. If Proxy
	// returns a nil *URL, no proxy is used.
	//
	// socks5 proxying does not currently support spdy streaming endpoints.
	Proxy func(*http.Request) (*url.URL, error)
}
