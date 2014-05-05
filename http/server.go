// Serve jobs over the http protocol, and provide a marshalling interface
// for the core geard jobs.
package http

import (
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"strings"

	"github.com/openshift/geard/config"
	cjobs "github.com/openshift/geard/containers/jobs"
	"github.com/openshift/geard/dispatcher"
	"github.com/openshift/geard/jobs"
	"github.com/openshift/go-json-rest"
)

func ApiVersion() string {
	return "1"
}

var ErrHandledResponse = errors.New("Request handled")

type HttpConfiguration struct {
	Docker     config.DockerConfiguration
	Dispatcher *dispatcher.Dispatcher
}

type JobHandler func(*jobs.JobContext, *rest.Request) (jobs.Job, error)

type HttpJobHandler interface {
	RemoteJob
	Handler(conf *HttpConfiguration) JobHandler
}

type HttpStreamable interface {
	Streamable() bool
}

func (conf *HttpConfiguration) Handler() http.Handler {
	handler := rest.ResourceHandler{
		EnableRelaxedContentType: true,
		EnableResponseStackTrace: true,
		EnableGzip:               false,
	}

	handlers := []HttpJobHandler{
		&HttpRunContainerRequest{},

		&HttpInstallContainerRequest{},
		&HttpDeleteContainerRequest{},
		&HttpContainerLogRequest{},
		&HttpContainerStatusRequest{},
		&HttpListContainerPortsRequest{},

		&HttpStartContainerRequest{},
		&HttpStopContainerRequest{},
		&HttpRestartContainerRequest{},

		&HttpLinkContainersRequest{},

		&HttpListContainersRequest{},
		&HttpListImagesRequest{},
		&HttpListBuildsRequest{},

		&HttpBuildImageRequest{},

		&HttpPatchEnvironmentRequest{},
		&HttpPutEnvironmentRequest{},

		&HttpContentRequest{},
		&HttpContentRequest{ContentRequest: cjobs.ContentRequest{Subpath: "*"}},
		&HttpContentRequest{ContentRequest: cjobs.ContentRequest{Type: cjobs.ContentTypeEnvironment}},
	}

	for _, ext := range extensions {
		routes := ext.Routes()
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
	return rest.Route{
		handler.HttpMethod(),
		handler.HttpPath(),
		conf.handleWithMethod(handler.Handler(conf)),
	}
}

func (conf *HttpConfiguration) handleWithMethod(method JobHandler) func(*rest.ResponseWriter, *rest.Request) {
	return func(w *rest.ResponseWriter, r *rest.Request) {
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

		context := &jobs.JobContext{}

		requestId := r.Header.Get("X-Request-Id")
		if requestId == "" {
			context.Id = jobs.NewRequestIdentifier()
		} else {
			id, err := jobs.NewRequestIdentifierFromString(requestId)
			if err != nil {
				http.Error(w, "X-Request-Id must be a 32 character hexadecimal string", http.StatusBadRequest)
				return
			}
			context.Id = id
		}

		/*token, id, errt := extractToken(r.PathParam("token"), r.Request)
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
		}*/

		job, errh := method(context, r)
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

		acceptHeader := r.Header.Get("Accept")
		overrideAcceptHeader := r.Header.Get("X-Accept")
		if overrideAcceptHeader != "" {
			acceptHeader = overrideAcceptHeader
		}
		canStream := didClientRequestStreamableResponse(acceptHeader)
		if streaming, ok := job.(HttpStreamable); ok {
			canStream = streaming.Streamable()
		}
		response := NewHttpJobResponse(w.ResponseWriter, !canStream, mode)

		wait, errd := conf.Dispatcher.Dispatch(context.Id, job, response)
		if errd == jobs.ErrRanToCompletion {
			http.Error(w, errd.Error(), http.StatusNoContent)
			return
		} else if errd != nil {
			serveRequestError(w, apiRequestError{errd, errd.Error(), http.StatusServiceUnavailable})
			return
		}
		<-wait
	}
}

func didClientRequestStreamableResponse(acceptHeader string) bool {
	result := false
	mediaTypes := strings.Split(acceptHeader, ",")
	for i := range mediaTypes {
		mediaType, params, _ := mime.ParseMediaType(mediaTypes[i])
		result = (params["stream"] == "true") && (mediaType == "application/json" || mediaType == "text/plain")
		if result {
			break
		}
	}
	return result
}

func limitedBodyReader(r *rest.Request) io.Reader {
	return io.LimitReader(r.Body, 100*1024)
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
