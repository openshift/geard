// Serve jobs over the http protocol, and provide a marshalling interface
// for the core geard jobs.
package http

import (
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"strings"

	"github.com/openshift/geard/config"
	"github.com/openshift/geard/dispatcher"
	"github.com/openshift/geard/jobs"
	"github.com/openshift/go-json-rest"
)

func ApiVersion() string {
	return "1"
}

type HttpConfiguration struct {
	Docker     config.DockerConfiguration
	Dispatcher *dispatcher.Dispatcher
}

type JobHandler func(*jobs.JobContext, *rest.Request) (interface{}, error)

type HttpJobHandler interface {
	RemoteJob
	Handler(conf *HttpConfiguration) JobHandler
}

type HttpStreamable interface {
	Streamable() bool
}

func (conf *HttpConfiguration) Handler() (http.Handler, error) {
	handler := rest.ResourceHandler{
		EnableRelaxedContentType: true,
		EnableResponseStackTrace: true,
		EnableGzip:               false,
	}

	handlers := []HttpJobHandler{}

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

	if err := handler.SetRoutes(routes...); err != nil {
		for i := range routes {
			log.Printf("failed: %+v", routes[i])
		}
		return nil, err
	}
	return &handler, nil
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

		// parse the incoming request into an object
		jobRequest, errh := method(context, r)
		if errh != nil {
			serveRequestError(w, apiRequestError{errh, errh.Error(), http.StatusBadRequest})
			return
		}

		// find the job implementation for that request
		job, errj := jobs.JobFor(jobRequest)
		if errj != nil {
			serveRequestError(w, apiRequestError{errj, errj.Error(), http.StatusBadRequest})
			return
		}

		// determine the type of the request
		acceptHeader := r.Header.Get("Accept")
		overrideAcceptHeader := r.Header.Get("X-Accept")
		if overrideAcceptHeader != "" {
			acceptHeader = overrideAcceptHeader
		}

		// setup the appropriate mode
		mode := ResponseJson
		if acceptHeader == "text/plain" {
			mode = ResponseTable
		}
		canStream := didClientRequestStreamableResponse(acceptHeader)
		response := NewHttpJobResponse(w.ResponseWriter, !canStream, mode)

		// queue / handle the request
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
