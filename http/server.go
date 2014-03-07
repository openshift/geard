package http

import (
	"errors"
	"fmt"
	"github.com/smarterclayton/geard/config"
	"github.com/smarterclayton/geard/dispatcher"
	"github.com/smarterclayton/geard/jobs"
	"github.com/smarterclayton/go-json-rest"
	"io"
	"log"
	"net/http"
	"strings"
)

func ApiVersion() string {
	return "1"
}

var ErrHandledResponse = errors.New("Request handled")

type HttpConfiguration struct {
	Docker     config.DockerConfiguration
	Dispatcher *dispatcher.Dispatcher
	Extensions []HttpExtension
}

type JobHandler func(jobs.RequestIdentifier, *TokenData, *rest.Request) (jobs.Job, error)

type HttpJobHandler interface {
	RemoteJob
	Handler(conf *HttpConfiguration) JobHandler
}

type HttpStreamable interface {
	Streamable() bool
}

type HttpExtension func() []HttpJobHandler

func (conf *HttpConfiguration) Handler() http.Handler {
	handler := rest.ResourceHandler{
		EnableRelaxedContentType: true,
		EnableResponseStackTrace: true,
		EnableGzip:               false,
	}

	handlers := []HttpJobHandler{
		&HttpInstallContainerRequest{},
		&HttpDeleteContainerRequest{},
		&HttpContainerLogRequest{},
		&HttpContainerStatusRequest{},
		&HttpListContainerPortsRequest{},

		&HttpStartContainerRequest{},
		&HttpStopContainerRequest{},

		&HttpLinkContainersRequest{},

		&HttpListContainersRequest{},
		&HttpListImagesRequest{},
		&HttpListBuildsRequest{},

		&HttpBuildImageRequest{},

		&HttpPatchEnvironmentRequest{},
		&HttpPutEnvironmentRequest{},

		&HttpContentRequest{},
		&HttpContentRequest{ContentRequest: jobs.ContentRequest{Subpath: "*"}},
		&HttpContentRequest{ContentRequest: jobs.ContentRequest{Type: jobs.ContentTypeEnvironment}},

		&HttpCreateKeysRequest{},
	}

	for i := range conf.Extensions {
		routes := conf.Extensions[i]()
		for j := range routes {
			handlers = append(handlers, routes[j])
		}
	}

	routes := make([]rest.Route, len(handlers))
	for i := range handlers {
		routes[i] = conf.jobRestHandler(handlers[i])
	}

	handler.SetRoutes(routes...)
	return &handler
}

func (conf *HttpConfiguration) jobRestHandler(handler HttpJobHandler) rest.Route {
	method := handler.Handler(conf)
	return rest.Route{
		handler.HttpMethod(),
		"/token/:token" + handler.HttpPath(),
		func(w *rest.ResponseWriter, r *rest.Request) {
			match := r.Header.Get("If-Match")
			segments := strings.Split(match, ",")
			for i := range segments {
				if strings.HasPrefix(segments[i], "api=") {
					if segments[i][4:] != ApiVersion() {
						http.Error(w, fmt.Sprintf("Current API version %s does not match requested %s", ApiVersion(), segments[i][4:]), http.StatusPreconditionFailed)
						return
					}
				}
			}

			token, id, errt := extractToken(r.PathParam("token"), r.Request)
			if errt != nil {
				log.Println(errt)
				http.Error(w, "Token is required - pass /token/<token>/<path>", http.StatusForbidden)
				return
			}

			if token.D == 0 {
				log.Println("http: Recommend passing 'd' as an argument for the current date")
			}
			if token.U == "" {
				log.Println("http: Recommend passing 'u' as an argument for the associated user")
			}

			job, errh := method(id, token, r)
			if errh != nil {
				if errh != ErrHandledResponse {
					http.Error(w, "Invalid request: "+errh.Error()+"\n", http.StatusBadRequest)
				}
				return
			}

			mode := ResponseJson
			if r.Header.Get("Accept") == "text/plain" {
				mode = ResponseTable
			}

			canStream := true
			if streaming, ok := job.(HttpStreamable); ok {
				canStream = streaming.Streamable()
			}
			response := NewHttpJobResponse(w.ResponseWriter, !canStream, mode)

			wait, errd := conf.Dispatcher.Dispatch(id, job, response)
			if errd == jobs.ErrRanToCompletion {
				http.Error(w, errd.Error(), http.StatusNoContent)
				return
			} else if errd != nil {
				serveRequestError(w, apiRequestError{errd, errd.Error(), http.StatusServiceUnavailable})
				return
			}
			<-wait
		},
	}
}

func limitedBodyReader(r *rest.Request) io.Reader {
	return io.LimitReader(r.Body, 100*1024)
}

func extractToken(segment string, r *http.Request) (token *TokenData, id jobs.RequestIdentifier, rerr *apiRequestError) {
	if segment == "__test__" {
		t, err := NewTokenFromMap(r.URL.Query())
		if err != nil {
			rerr = &apiRequestError{err, "Invalid test query: " + err.Error(), http.StatusForbidden}
			return
		}
		token = t
	} else {
		t, err := NewTokenFromString(segment)
		if err != nil {
			rerr = &apiRequestError{err, "Invalid authorization token", http.StatusForbidden}
			return
		}
		token = t
	}

	if token.I == "" {
		id = jobs.NewRequestIdentifier()
	} else {
		i, errr := token.RequestId()
		if errr != nil {
			rerr = &apiRequestError{errr, "Unable to parse token for this request: " + errr.Error(), http.StatusBadRequest}
			return
		}
		id = i
	}

	return
}

type apiRequestError struct {
	Error   error
	Message string
	Status  int
}

func serveRequestError(w http.ResponseWriter, err apiRequestError) {
	log.Print(err.Message, err.Error)
	http.Error(w, err.Message, err.Status)
}
