package streaming

import (
	"github.com/containerd/containerd/api/services/tasks/v1"
	"github.com/emicklei/go-restful"
	"net/http"
	"net/url"
	"path"
)

// Server is the library interface to serve the stream requests.
type Server interface {
	http.Handler

	// Get the serving URL for the requests.
	// Requests must not be nil. Responses may be nil iff an error is returned.
	GetCreateTask(*tasks.CreateTaskRequest) (string, error)
}

type server struct {
	handler http.Handler
	server  *http.Server

	cache *requestCache
}

// NewServer creates a new Server for stream requests.
// TODO(tallclair): Add auth(n/z) interface & handling.
func NewServer() (Server, error) {
	s := &server{
		cache: newRequestCache(),
	}

	ws := &restful.WebService{}
	endpoints := []struct {
		path    string
		handler restful.RouteFunction
	}{
		{"/exec/{token}", s.serveExec},
	}
	// If serving relative to a base path, set that here.
	pathPrefix := path.Dir("/")
	for _, e := range endpoints {
		for _, method := range []string{"GET", "POST"} {
			ws.Route(ws.
				Method(method).
				Path(path.Join(pathPrefix, e.path)).
				To(e.handler))
		}
	}
	handler := restful.NewContainer()
	handler.Add(ws)
	s.handler = handler

	return s, nil
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("eddycjy: go-grpc-example\n"))
}

func (s *server) GetCreateTask(req *tasks.CreateTaskRequest) (string, error) {
	token, err := s.cache.Insert(req)
	if err != nil {
		return "", err
	}
	return s.buildURL("exec", token), nil
}

func (s *server) buildURL(method, token string) string {
	return (&url.URL{
		Path: path.Join("/", method, token),
	}).String()
}

func (s *server) serveExec(req *restful.Request, resp *restful.Response) {
	//token := req.PathParameter("token")
	//cachedRequest, ok := s.cache.Consume(token)
	//if !ok {
	//	http.NotFound(resp.ResponseWriter, req.Request)
	//	return
	//}
	//exec, ok := cachedRequest.(*runtimeapi.ExecRequest)
	//if !ok {
	//	http.NotFound(resp.ResponseWriter, req.Request)
	//	return
	//}
	//
	//streamOpts := &remotecommandserver.Options{
	//	Stdin:  exec.Stdin,
	//	Stdout: exec.Stdout,
	//	Stderr: exec.Stderr,
	//	TTY:    exec.Tty,
	//}
	//
	//remotecommandserver.ServeExec(
	//	resp.ResponseWriter,
	//	req.Request,
	//	s.runtime,
	//	"", // unused: podName
	//	"", // unusued: podUID
	//	exec.ContainerId,
	//	exec.Cmd,
	//	streamOpts,
	//	s.config.StreamIdleTimeout,
	//	s.config.StreamCreationTimeout,
	//	s.config.SupportedRemoteCommandProtocols)
}
