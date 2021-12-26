package remotecommand

import (
	contextpkg "context"
	"github.com/containerd/containerd/services/streaming/remotecommand/httpstream"
	restclient "github.com/containerd/containerd/services/streaming/remotecommand/rest"
	"github.com/containerd/containerd/services/streaming/remotecommand/transport"
	gwebsocket "github.com/gorilla/websocket"
	"io"
	"k8s.io/apimachinery/pkg/util/remotecommand"
	"k8s.io/klog/v2"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// StreamOptions holds information pertaining to the current streaming session:
// input/output streams, if the client is requesting a TTY, and a terminal size queue to
// support terminal resizing.
type StreamOptions struct {
	Stdin             io.Reader
	Stdout            io.Writer
	Stderr            io.Writer
	Tty               bool
	TerminalSizeQueue TerminalSizeQueue
}

// Executor is an interface for transporting shell-style streams.
type Executor interface {
	// Stream initiates the transport of the standard shell streams. It will transport any
	// non-nil stream to a remote system, and return an error if a problem occurs. If tty
	// is set, the stderr stream is not used (raw TTY manages stdout and stderr over the
	// stdout stream).
	Stream(options StreamOptions) error
}

// streamExecutor handles transporting standard shell streams over an httpstream connection.
type streamExecutor struct {
	upgrader  transport.Upgrader
	transport http.RoundTripper

	wsRoundTripper httpstream.RoundTripper

	url       *url.URL
	address   string
	protocols []string
}

type streamProtocolHandler interface {
	stream(conn *gwebsocket.Conn) error
}

// NewWebSocketExecutor creates a new websocket connection to the URL specified with
// the rest client's TLS configuration and headers
func NewWebSocketExecutor(config *restclient.Config, url *url.URL, address string) (Executor, error) {
	url.Scheme = "ws"

	wrapper, upgradeRoundTripper, err := transport.RoundTripperFor(config)
	if err != nil {
		return nil, err
	}

	return NewWebSocketExecutorForTransports(wrapper, upgradeRoundTripper, url, address)
}

// NewWebSocketExecutorForTransports connects to the provided server using the given transport,
// upgrades the response using the given upgrader to multiplexed bidirectional streams.
func NewWebSocketExecutorForTransports(transport http.RoundTripper, upgrader transport.Upgrader, url *url.URL, address string) (Executor, error) {
	return NewWebSocketExecutorForProtocols(
		transport, upgrader, url,
		address,
		v4BinaryWebsocketProtocol,
	)
}

// NewWebSocketExecutorForProtocols connects to the provided server and upgrades the connection to
// multiplexed bidirectional streams using only the provided protocols. Exposed for testing, most
// callers should use NewWebSocketExecutor or NewWebSocketExecutorForTransports.
func NewWebSocketExecutorForProtocols(transport http.RoundTripper, upgrader transport.Upgrader, url *url.URL, address string, protocols ...string) (Executor, error) {
	return &streamExecutor{
		upgrader:  upgrader,
		transport: transport,
		url:       url,
		address:   address,
		protocols: protocols,
	}, nil
}

// Stream opens a protocol streamer to the server and streams until a client closes
// the connection or the server disconnects.
func (e *streamExecutor) Stream(options StreamOptions) error {
	dialer := gwebsocket.Dialer{
		NetDialContext: func(ctx contextpkg.Context, network, addr string) (net.Conn, error) {
			parts := strings.SplitN(e.address, "://", 2)
			// todo tls dial
			conn, err := net.Dial(parts[0], parts[1])
			return conn, err
		},
	}

	headers := http.Header{
		transport.SecWebsocketProtocol: []string{
			// todo debug, 后续应该优先使用binary协议
			v4Base64WebsocketProtocol,
			v4BinaryWebsocketProtocol,
			preV4BinaryWebsocketProtocol,
			preV4Base64WebsocketProtocol,
		},
	}
	conn, resp, err := dialer.Dial(e.url.String(), headers)
	if err != nil {
		return err
	}
	protocol := resp.Header.Get(transport.SecWebsocketProtocol)
	klog.V(4).Infof("The protocol is  %s", protocol)

	var streamer streamProtocolHandler
	switch protocol {
	case v4BinaryWebsocketProtocol:
		streamer = newBinaryV4(options)
	case v4Base64WebsocketProtocol:
		streamer = newBase64V4(options)
	case preV4Base64WebsocketProtocol:
		streamer = newPreV4Base64Protocol(options)
	case "":
		klog.V(4).Infof("The server did not negotiate a streaming protocol version. Falling back to %s", remotecommand.StreamProtocolV1Name)
		fallthrough
	case preV4BinaryWebsocketProtocol:
		streamer = newPreV4BinaryProtocol(options)
	}

	return streamer.stream(conn)
}
