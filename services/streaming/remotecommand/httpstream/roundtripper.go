package httpstream

import (
	"crypto/tls"
	"github.com/gorilla/websocket"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"net/http"
	"net/url"
)

// RoundTripper stores dialer information and knows how
// to establish a connect to the remote websocket endpoint.  WebsocketRoundTripper
// implements the UpgradeRoundTripper interface.
type RoundTripper struct {
	http.RoundTripper
	//tlsConfig holds the TLS configuration settings to use when connecting
	//to the remote server.
	tlsConfig *tls.Config

	// websocket connection
	Conn *websocket.Conn

	// proxier knows which proxy to use given a request, defaults to http.ProxyFromEnvironment
	// Used primarily for mocking the proxy discovery in tests.
	proxier func(req *http.Request) (*url.URL, error)

	// followRedirects indicates if the round tripper should examine responses for redirects and
	// follow them.
	followRedirects bool
	// requireSameHostRedirects restricts redirect following to only follow redirects to the same host
	// as the original request.
	requireSameHostRedirects bool
}

var _ UpgradeRoundTripper = &RoundTripper{}

// Connection holds the underlying websocket connection
type WsConnection struct {
	httpstream.Connection
	Conn *websocket.Conn

	//store streams by channel id
	channels map[int]httpstream.Stream
}

func (w WsConnection) CreateStream(headers http.Header) (Stream, error) {
	//TODO implement me
	panic("implement me")
}

func (w WsConnection) RemoveStreams(streams ...Stream) {
	//TODO implement me
	panic("implement me")
}

// NewRoundTripperWithProxy creates a new WsRoundTripper that will use the
// specified tlsConfig and proxy func.
func NewRoundTripperWithProxy(tlsConfig *tls.Config, followRedirects, requireSameHostRedirects bool, proxier func(*http.Request) (*url.URL, error)) *RoundTripper {
	return &RoundTripper{
		tlsConfig:                tlsConfig,
		followRedirects:          followRedirects,
		requireSameHostRedirects: requireSameHostRedirects,
		proxier:                  proxier,
	}
}

// NewConnection doesn't do anything right now
func (wsRoundTripper *RoundTripper) NewConnection(resp *http.Response) (Connection, error) {
	return &WsConnection{Conn: wsRoundTripper.Conn}, nil
}
