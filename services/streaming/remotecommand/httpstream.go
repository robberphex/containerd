package remotecommand

import (
	"encoding/json"
	"io"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/util/wsstream"
	"k8s.io/client-go/tools/remotecommand"
	"net/http"
	"time"
)

// Options contains details about which streams are required for
// remote command execution.
type Options struct {
	Stdin  bool
	Stdout bool
	Stderr bool
	TTY    bool
}

// context contains the connection and streams used when
// forwarding an attach or execute session into a container.
type context struct {
	conn         io.Closer
	stdinStream  io.ReadCloser
	stdoutStream io.WriteCloser
	stderrStream io.WriteCloser
	writeStatus  func(status *apierrors.StatusError) error
	resizeStream io.ReadCloser
	resizeChan   chan remotecommand.TerminalSize
	tty          bool
}

// v4WriteStatusFunc returns a WriteStatusFunc that marshals a given api Status
// as json in the error channel.
func v4WriteStatusFunc(stream io.Writer) func(status *apierrors.StatusError) error {
	return func(status *apierrors.StatusError) error {
		bs, err := json.Marshal(status.Status())
		if err != nil {
			return err
		}
		_, err = stream.Write(bs)
		return err
	}
}

func v1WriteStatusFunc(stream io.Writer) func(status *apierrors.StatusError) error {
	return func(status *apierrors.StatusError) error {
		if status.Status().Status == metav1.StatusSuccess {
			return nil // send error messages
		}
		_, err := stream.Write([]byte(status.Error()))
		return err
	}
}

func createStreams(req *http.Request, w http.ResponseWriter, opts *Options, idleTimeout time.Duration) (*context, bool) {
	var ctx *context
	var ok bool
	if wsstream.IsWebSocketRequest(req) {
		ctx, ok = createWebSocketStreams(req, w, opts, idleTimeout)
	} else {
		return ctx, false
	}
	if !ok {
		return nil, false
	}

	if ctx.resizeStream != nil {
		ctx.resizeChan = make(chan remotecommand.TerminalSize)
		go handleResizeEvents(ctx.resizeStream, ctx.resizeChan)
	}

	return ctx, true
}

func handleResizeEvents(stream io.Reader, channel chan<- remotecommand.TerminalSize) {
	defer runtime.HandleCrash()
	defer close(channel)

	decoder := json.NewDecoder(stream)
	for {
		size := remotecommand.TerminalSize{}
		if err := decoder.Decode(&size); err != nil {
			break
		}
		channel <- size
	}
}
