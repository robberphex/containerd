package httpstream

import (
	"io"
	"net/http"
	"time"
)

const (
	HeaderUpgrade         = "Upgrade"
	HeaderProtocolVersion = "X-Stream-Protocol-Version"
)

// Connection represents an upgraded HTTP connection.
type Connection interface {
	// CreateStream creates a new Stream with the supplied headers.
	CreateStream(headers http.Header) (Stream, error)
	// Close resets all streams and closes the connection.
	Close() error
	// CloseChan returns a channel that is closed when the underlying connection is closed.
	CloseChan() <-chan bool
	// SetIdleTimeout sets the amount of time the connection may remain idle before
	// it is automatically closed.
	SetIdleTimeout(timeout time.Duration)
	// RemoveStreams can be used to remove a set of streams from the Connection.
	RemoveStreams(streams ...Stream)
}

// Stream represents a bidirectional communications channel that is part of an
// upgraded connection.
type Stream interface {
	io.ReadWriteCloser
	// Reset closes both directions of the stream, indicating that neither client
	// or server can use it any more.
	Reset() error
	// Headers returns the headers used to create the stream.
	Headers() http.Header
	// Identifier returns the stream's ID.
	Identifier() uint32
}

// UpgradeRoundTripper is a type of http.RoundTripper that is able to upgrade
// HTTP requests to support multiplexed bidirectional streams. After RoundTrip()
// is invoked, if the upgrade is successful, clients may retrieve the upgraded
// connection by calling UpgradeRoundTripper.Connection().
type UpgradeRoundTripper interface {
	http.RoundTripper
	// NewConnection validates the response and creates a new Connection.
	NewConnection(resp *http.Response) (Connection, error)
}
