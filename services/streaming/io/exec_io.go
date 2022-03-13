package io

import (
	"github.com/containerd/containerd/cio"
	cioutil "github.com/containerd/containerd/pkg/ioutil"
	"github.com/sirupsen/logrus"
	"io"
	"sync"
)

// ExecIO holds the exec io.
type ExecIO struct {
	id    string
	fifos *cio.FIFOSet
	*stdioPipes
	closer *wgCloser
}

var _ cio.IO = &ExecIO{}

// NewExecIO creates exec io.
func NewExecIO(id, root string, tty, stdin bool) (*ExecIO, error) {
	fifos, err := newFifos(root, id, tty, stdin)
	if err != nil {
		return nil, err
	}
	stdio, closer, err := NewStdioPipes(fifos)
	if err != nil {
		return nil, err
	}
	return &ExecIO{
		id:         id,
		fifos:      fifos,
		stdioPipes: stdio,
		closer:     closer,
	}, nil
}

// Config returns io config.
func (e *ExecIO) Config() cio.Config {
	return e.fifos.Config
}

// Attach attaches exec stdio. The logic is similar with container io attach.
func (e *ExecIO) Attach(opts AttachOptions) <-chan struct{} {
	var wg sync.WaitGroup
	var stdinStreamRC io.ReadCloser
	if e.Stdin != nil && opts.Stdin != nil {
		stdinStreamRC = cioutil.NewWrapReadCloser(opts.Stdin)
		wg.Add(1)
		go func() {
			if _, err := io.Copy(e.Stdin, stdinStreamRC); err != nil {
				logrus.WithError(err).Errorf("Failed to redirect Stdin for container exec %q", e.id)
			}
			logrus.Infof("Container exec %q Stdin closed", e.id)
			if opts.StdinOnce && !opts.Tty {
				e.Stdin.Close()
				if err := opts.CloseStdin(); err != nil {
					logrus.WithError(err).Errorf("Failed to close Stdin for container exec %q", e.id)
				}
			} else {
				if e.Stdout != nil {
					e.Stdout.Close()
				}
				if e.Stderr != nil {
					e.Stderr.Close()
				}
			}
			wg.Done()
		}()
	}

	attachOutput := func(t StreamType, stream io.WriteCloser, out io.ReadCloser) {
		if _, err := io.Copy(stream, out); err != nil {
			logrus.WithError(err).Errorf("Failed to pipe %q for container exec %q", t, e.id)
		}
		out.Close()
		stream.Close()
		if stdinStreamRC != nil {
			stdinStreamRC.Close()
		}
		e.closer.wg.Done()
		wg.Done()
		logrus.Debugf("Finish piping %q of container exec %q", t, e.id)
	}

	if opts.Stdout != nil {
		wg.Add(1)
		// Closer should wait for this routine to be over.
		e.closer.wg.Add(1)
		go attachOutput(Stdout, opts.Stdout, e.Stdout)
	}

	if !opts.Tty && opts.Stderr != nil {
		wg.Add(1)
		// Closer should wait for this routine to be over.
		e.closer.wg.Add(1)
		go attachOutput(Stderr, opts.Stderr, e.Stderr)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	return done
}

// Cancel cancels exec io.
func (e *ExecIO) Cancel() {
	e.closer.Cancel()
}

// Wait waits exec io to finish.
func (e *ExecIO) Wait() {
	e.closer.Wait()
}

// Close closes all FIFOs.
func (e *ExecIO) Close() error {
	if e.closer != nil {
		e.closer.Close()
	}
	if e.fifos != nil {
		return e.fifos.Close()
	}
	return nil
}