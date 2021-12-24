package streaming

import (
	"github.com/containerd/containerd/api/services/tasks/v1"
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
