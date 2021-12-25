/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package remotecommand

import (
	"fmt"
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/server/httplog"
	"k8s.io/apiserver/pkg/util/wsstream"
)

const (
	stdinChannel = iota
	stdoutChannel
	stderrChannel
	errorChannel
	resizeChannel
)

const (
	// StreamStdIn represents the remote stdin stream
	StreamStdIn = 0
	// StreamStdOut represents the remote stdout stream
	StreamStdOut = 1
	// StreamStdErr represents the remote stderr stream
	StreamStdErr = 2
	// StreamErr respresents the remote error stream
	StreamErr = 3
	// StreamResize represents the resize stream
	StreamResize = 4

	// Base64StreamStdIn represents the remote stdin stream
	Base64StreamStdIn = 48
	// Base64StreamStdOut represents the remote stdout stream
	Base64StreamStdOut = 49
	// Base64StreamStdErr represents the remote stderr stream
	Base64StreamStdErr = 50
	// Base64StreamErr respresents the remote error stream
	Base64StreamErr = 51
	// Base64StreamResize represents the resize stream
	Base64StreamResize = 52

	// WebSocketExitStream is the error code that represents a clean exit from the stream
	WebSocketExitStream = 1000

	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Maximum message size allowed from peer.
	maxMessageSize = 8192

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Time to wait before force close on connection.
	closeGracePeriod = 10 * time.Second

	preV4BinaryWebsocketProtocol = "channel.k8s.io"
	preV4Base64WebsocketProtocol = "base64.channel.k8s.io"
	v4BinaryWebsocketProtocol    = "v4." + preV4BinaryWebsocketProtocol
	v4Base64WebsocketProtocol    = "v4." + preV4Base64WebsocketProtocol
)

// createChannels returns the standard channel types for a shell connection (STDIN 0, STDOUT 1, STDERR 2)
// along with the approximate duplex value. It also creates the error (3) and resize (4) channels.
func createChannels(opts *Options) []wsstream.ChannelType {
	// open the requested channels, and always open the error channel
	channels := make([]wsstream.ChannelType, 5)
	channels[stdinChannel] = readChannel(opts.Stdin)
	channels[stdoutChannel] = writeChannel(opts.Stdout)
	channels[stderrChannel] = writeChannel(opts.Stderr)
	channels[errorChannel] = wsstream.WriteChannel
	channels[resizeChannel] = wsstream.ReadChannel
	return channels
}

// readChannel returns wsstream.ReadChannel if real is true, or wsstream.IgnoreChannel.
func readChannel(real bool) wsstream.ChannelType {
	if real {
		return wsstream.ReadChannel
	}
	return wsstream.IgnoreChannel
}

// writeChannel returns wsstream.WriteChannel if real is true, or wsstream.IgnoreChannel.
func writeChannel(real bool) wsstream.ChannelType {
	if real {
		return wsstream.WriteChannel
	}
	return wsstream.IgnoreChannel
}

// createWebSocketStreams returns a context containing the websocket connection and
// streams needed to perform an exec or an attach.
func createWebSocketStreams(req *http.Request, w http.ResponseWriter, opts *Options, idleTimeout time.Duration) (*context, bool) {
	channels := createChannels(opts)
	conn := wsstream.NewConn(map[string]wsstream.ChannelProtocolConfig{
		"": {
			Binary:   true,
			Channels: channels,
		},
		preV4BinaryWebsocketProtocol: {
			Binary:   true,
			Channels: channels,
		},
		preV4Base64WebsocketProtocol: {
			Binary:   false,
			Channels: channels,
		},
		v4BinaryWebsocketProtocol: {
			Binary:   true,
			Channels: channels,
		},
		v4Base64WebsocketProtocol: {
			Binary:   false,
			Channels: channels,
		},
	})
	conn.SetIdleTimeout(idleTimeout)
	negotiatedProtocol, streams, err := conn.Open(httplog.Unlogged(req, w), req)
	if err != nil {
		runtime.HandleError(fmt.Errorf("unable to upgrade websocket connection: %v", err))
		return nil, false
	}

	// Send an empty message to the lowest writable channel to notify the client the connection is established
	// TODO: make generic to SPDY and WebSockets and do it outside of this method?
	switch {
	case opts.Stdout:
		streams[stdoutChannel].Write([]byte{})
	case opts.Stderr:
		streams[stderrChannel].Write([]byte{})
	default:
		streams[errorChannel].Write([]byte{})
	}

	ctx := &context{
		conn:         conn,
		stdinStream:  streams[stdinChannel],
		stdoutStream: streams[stdoutChannel],
		stderrStream: streams[stderrChannel],
		tty:          opts.TTY,
		resizeStream: streams[resizeChannel],
	}

	switch negotiatedProtocol {
	case v4BinaryWebsocketProtocol, v4Base64WebsocketProtocol:
		ctx.writeStatus = v4WriteStatusFunc(streams[errorChannel])
	default:
		ctx.writeStatus = v1WriteStatusFunc(streams[errorChannel])
	}

	return ctx, true
}
