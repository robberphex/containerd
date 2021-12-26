package transport

import (
	"crypto/tls"
	"fmt"
	"github.com/containerd/containerd/services/streaming/remotecommand/httpstream"
	restclient "github.com/containerd/containerd/services/streaming/remotecommand/rest"
	"net/http"
)

const (
	// SecWebsocketProtocol is the response header from the API server
	// that tells us which exec protocol to use
	SecWebsocketProtocol = "Sec-Websocket-Protocol"
)

// Upgrader validates a response from the server after a WebSocket upgrade.
type Upgrader interface {
	// NewConnection validates the response and creates a new Connection.
	NewConnection(resp *http.Response) (httpstream.Connection, error)
}

// RoundTripperFor creates a RountTripper wrapper so the websocket dialer
// can be properly setup.
func RoundTripperFor(config *restclient.Config) (http.RoundTripper, Upgrader, error) {

	//get the rest configuration's
	//tlsConfig, err := restclient.TLSConfigFor(config)
	//if err != nil {
	//	return nil, nil, err
	//}
	var tlsConfig *tls.Config
	proxy := http.ProxyFromEnvironment
	if config.Proxy != nil {
		proxy = config.Proxy
	}

	upgradeRoundTripper := httpstream.NewRoundTripperWithProxy(tlsConfig, true, false, proxy)
	//wrapper, err := restclient.HTTPWrappersForConfig(config, upgradeRoundTripper)
	//if err != nil {
	//	return nil, nil, err
	//}
	return upgradeRoundTripper, upgradeRoundTripper, nil

}

// Negotiate opens a connection to a remote server and attempts to negotiate
// a WebSocket connection. Upon success, it returns the connection and the protocol selected by
// the server. The client transport must use the upgradeRoundTripper - see RoundTripperFor.
func Negotiate(upgrader Upgrader, client *http.Client, req *http.Request, protocols ...string) (httpstream.Connection, string, error) {
	for i := range protocols {
		req.Header.Add(httpstream.HeaderProtocolVersion, protocols[i])
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()
	conn, err := upgrader.NewConnection(resp)
	if err != nil {
		return nil, "", err
	}
	return conn, resp.Header.Get(SecWebsocketProtocol), nil
}
