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
	"github.com/containerd/containerd/api/services/tasks/v1"
	"github.com/containerd/containerd/cio"
	cioutil "github.com/containerd/containerd/pkg/ioutil"
	streamingIO "github.com/containerd/containerd/services/streaming/io"
	"github.com/sirupsen/logrus"
	"io"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	remotecommandconsts "k8s.io/apimachinery/pkg/util/remotecommand"
	"k8s.io/apimachinery/pkg/util/runtime"
	utilexec "k8s.io/utils/exec"
	"net/http"
	"sync"
	"time"
)

// ServeExec handles requests to execute a command in a container. After
// creating/receiving the required streams, it delegates the actual execution
// to the executor.
func ServeExec(
	w http.ResponseWriter,
	req *http.Request,
	createTaskRequest *tasks.CreateTaskRequest,
	//executor Executor,
	streamOpts *Options,
	//idleTimeout, streamCreationTimeout time.Duration, supportedProtocols []string
) {
	ctx, ok := createStreams(req, w, streamOpts, 2*time.Hour)
	if !ok {
		// error is handled by createStreams
		return
	}
	defer ctx.conn.Close()

	err := execStream(ctx, createTaskRequest)
	//err = executor.ExecInContainer(podName, uid, container, cmd, ctx.stdinStream, ctx.stdoutStream, ctx.stderrStream, ctx.tty, ctx.resizeChan, 0)
	if err != nil {
		if exitErr, ok := err.(utilexec.ExitError); ok && exitErr.Exited() {
			rc := exitErr.ExitStatus()
			ctx.writeStatus(&apierrors.StatusError{ErrStatus: metav1.Status{
				Status: metav1.StatusFailure,
				Reason: remotecommandconsts.NonZeroExitCodeReason,
				Details: &metav1.StatusDetails{
					Causes: []metav1.StatusCause{
						{
							Type:    remotecommandconsts.ExitCodeCauseType,
							Message: fmt.Sprintf("%d", rc),
						},
					},
				},
				Message: fmt.Sprintf("command terminated with non-zero exit code: %v", exitErr),
			}})
		} else {
			err = fmt.Errorf("error executing command in container: %v", err)
			runtime.HandleError(err)
			ctx.writeStatus(apierrors.NewInternalError(err))
		}
	} else {
		ctx.writeStatus(&apierrors.StatusError{ErrStatus: metav1.Status{
			Status: metav1.StatusSuccess,
		}})
	}
}

func execStream(ctx *context, createTaskRequest *tasks.CreateTaskRequest) error {
	stdio, _, err := streamingIO.NewStdioPipes(&cio.FIFOSet{
		Config: cio.Config{
			Terminal: true,
			Stdin:    createTaskRequest.Stdin,
			Stdout:   createTaskRequest.Stdout,
			Stderr:   createTaskRequest.Stderr,
		},
	})
	if err != nil {
		return err
	}
	var stdinStreamRC io.ReadCloser
	// attach逻辑
	var wg sync.WaitGroup
	// region stdin
	stdinStreamRC = cioutil.NewWrapReadCloser(ctx.stdinStream)
	wg.Add(1)
	go func() {
		if _, err := io.Copy(stdio.Stdin, stdinStreamRC); err != nil {
			logrus.WithError(err).Errorf("Failed to redirect stdin for container exec %%q")
		}
		logrus.Infof("Container exec %%q stdin closed")
		wg.Done()
	}()
	// endregion
	attachOutput := func(stream io.WriteCloser, out io.ReadCloser) {
		if _, err := io.Copy(stream, out); err != nil {
			logrus.WithError(err).Errorf("Failed to pipe %%q for container exec %%q")
		}
		out.Close()
		stream.Close()
		if stdinStreamRC != nil {
			stdinStreamRC.Close()
		}
		wg.Done()
		logrus.Debugf("Finish piping %%q of container exec %%q")
	}
	// region stdout
	wg.Add(1)
	// Closer should wait for this routine to be over.
	go attachOutput(ctx.stdoutStream, stdio.Stdout)
	// endregion
	// region stderr
	wg.Add(1)
	// Closer should wait for this routine to be over.
	go attachOutput(ctx.stderrStream, stdio.Stderr)
	// endregion
	wg.Wait()
	return nil
}
