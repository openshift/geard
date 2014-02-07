package geard

import (
	"errors"
	//"fmt"
	"encoding/json"
	"github.com/ant0ine/go-json-rest"
	"io"
	"log"
	"net/http"
)

var ErrHandledResponse = errors.New("Request handled")

func NewHttpApiHandler(dispatcher *Dispatcher) *rest.ResourceHandler {
	handler := rest.ResourceHandler{
		EnableRelaxedContentType: true,
		EnableResponseStackTrace: true,
	}
	handler.SetRoutes(
		rest.Route{"PUT", "/token/:token/container", JobRestHandler(dispatcher, ApiPutContainer)},
		rest.Route{"PUT", "/token/:token/container/:action", JobRestHandler(dispatcher, ApiPutContainerAction)},
		rest.Route{"PUT", "/token/:token/repository", JobRestHandler(dispatcher, ApiPutRepository)},
		rest.Route{"GET", "/token/:token/content/*", JobRestHandler(dispatcher, ApiGetContent)},
	)
	return &handler
}

type JobHandler func(RequestIdentifier, *TokenData, *rest.ResponseWriter, *rest.Request) (Job, error)

func JobRestHandler(dispatcher *Dispatcher, handler JobHandler) func(*rest.ResponseWriter, *rest.Request) {
	return func(w *rest.ResponseWriter, r *rest.Request) {
		token, id, errt := extractToken(r.PathParam("token"), r.Request)
		if errt != nil {
			log.Println(errt)
			http.Error(w, "Token is required - pass /token/<token>/<path>", http.StatusForbidden)
			return
		}

		if token.U == "" {
			http.Error(w, "All requests must be associated with a user", http.StatusBadRequest)
			return
		}

		job, errh := handler(id, token, w, r)
		if errh != nil {
			if errh != ErrHandledResponse {
				http.Error(w, "Invalid request: "+errh.Error()+"\n", http.StatusBadRequest)
			}
			return
		}

		wait, errd := dispatcher.Dispatch(job)
		if errd == ErrRanToCompletion {
			http.Error(w, errd.Error(), http.StatusNoContent)
			return
		} else if errd != nil {
			serveRequestError(w, apiRequestError{errd, errd.Error(), http.StatusServiceUnavailable})
			return
		}
		<-wait
	}
}

func ApiPutContainer(reqid RequestIdentifier, token *TokenData, w *rest.ResponseWriter, r *rest.Request) (Job, error) {
	gearId, errg := NewGearIdentifier(token.ResourceLocator())
	if errg != nil {
		return nil, errg
	}
	if token.ResourceType() == "" {
		return nil, errors.New("A container must have an image identifier")
	}

	data := extendedCreateContainerData{}
	if r.Body != nil {
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&data); err != nil && err != io.EOF {
			return nil, err
		}
	}
	if data.Ports == nil {
		data.Ports = make([]PortPair, 0)
	}

	return &createContainerJobRequest{jobRequest{reqid}, gearId, token.U, token.ResourceType(), w, &data}, nil
}

func ApiPutRepository(reqid RequestIdentifier, token *TokenData, w *rest.ResponseWriter, r *rest.Request) (Job, error) {
	repositoryId, errg := NewGearIdentifier(token.ResourceLocator())
	if errg != nil {
		return nil, errg
	}
	return &createRepositoryJobRequest{jobRequest{reqid}, repositoryId, token.U, "ccoleman/githost", w}, nil
}

func ApiPutContainerAction(reqid RequestIdentifier, token *TokenData, w *rest.ResponseWriter, r *rest.Request) (Job, error) {
	action := r.PathParam("action")
	gearId, errg := NewGearIdentifier(token.ResourceLocator())
	if errg != nil {
		return nil, errg
	}
	switch action {
	case "started":
		return &startedContainerStateJobRequest{jobRequest{reqid}, gearId, token.U, w}, nil
	case "stopped":
		return &stoppedContainerStateJobRequest{jobRequest{reqid}, gearId, token.U, w}, nil
	default:
		return nil, errors.New("You must provide a valid action for this container to take")
	}
}

func ApiGetContent(reqid RequestIdentifier, token *TokenData, w *rest.ResponseWriter, r *rest.Request) (Job, error) {
	if token.ResourceLocator() == "" {
		return nil, errors.New("You must specify the location of the content you want to access")
	}
	if token.ResourceType() == "" {
		return nil, errors.New("You must specify the type of the content you want to access")
	}

	return &contentJobRequest{jobRequest{reqid}, token.ResourceType(), token.ResourceLocator(), r.PathParam("*"), w}, nil
}

func extractToken(segment string, r *http.Request) (token *TokenData, id RequestIdentifier, rerr *apiRequestError) {
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

	i, err := token.RequestId()
	if err != nil {
		rerr = &apiRequestError{err, "Token is missing data: " + err.Error(), http.StatusBadRequest}
		return
	}
	id = i

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
