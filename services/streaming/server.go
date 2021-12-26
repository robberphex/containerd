package streaming

import (
	"github.com/containerd/containerd/api/services/tasks/v1"
	"github.com/containerd/containerd/services/streaming/remotecommand"
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
	GetCreateTask(r *tasks.CreateTaskRequest) (string, error)
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
	s.handler.ServeHTTP(w, r)
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
	token := req.PathParameter("token")
	cachedRequest, ok := s.cache.Consume(token)
	if !ok {
		http.NotFound(resp.ResponseWriter, req.Request)
		return
	}
	createTaskRequest, ok := cachedRequest.(*tasks.CreateTaskRequest)
	if !ok {
		http.NotFound(resp.ResponseWriter, req.Request)
		return
	}

	streamOpts := &remotecommand.Options{
		Stdin:  createTaskRequest.Stdin != "",
		Stdout: createTaskRequest.Stdout != "",
		Stderr: createTaskRequest.Stderr != "",
		TTY:    createTaskRequest.Terminal,
	}

	remotecommand.ServeExec(
		resp.ResponseWriter,
		req.Request,
		createTaskRequest,
		streamOpts,
	)
}
