package geard

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"github.com/smarterclayton/go-json-rest"
	"io"
	"log"
	"net/http"
	"net/url"
)

var ErrHandledResponse = errors.New("Request handled")

func NewHttpApiHandler(dispatcher *Dispatcher) *rest.ResourceHandler {
	handler := rest.ResourceHandler{
		EnableRelaxedContentType: true,
		EnableResponseStackTrace: true,
		EnableGzip:               false,
	}
	handler.SetRoutes(
		rest.Route{"GET", "/token/:token/images", JobRestHandler(dispatcher, ApiListImages)},
		rest.Route{"GET", "/token/:token/containers", JobRestHandler(dispatcher, ApiListContainers)},
		rest.Route{"PUT", "/token/:token/container", JobRestHandler(dispatcher, ApiPutContainer)},
		rest.Route{"GET", "/token/:token/container/log", JobRestHandler(dispatcher, ApiGetContainerLog)},
		rest.Route{"PUT", "/token/:token/container/:action", JobRestHandler(dispatcher, ApiPutContainerAction)},
		rest.Route{"PUT", "/token/:token/repository", JobRestHandler(dispatcher, ApiPutRepository)},
		rest.Route{"PUT", "/token/:token/keys", JobRestHandler(dispatcher, ApiPutKeys)},
		rest.Route{"GET", "/token/:token/content", JobRestHandler(dispatcher, ApiGetContent)},
		rest.Route{"GET", "/token/:token/content/*", JobRestHandler(dispatcher, ApiGetContent)},
		rest.Route{"PUT", "/token/:token/build-image", JobRestHandler(dispatcher, ApiPutBuildImageAction)},
		rest.Route{"PUT", "/token/:token/environment", JobRestHandler(dispatcher, ApiPutEnvironment)},
		rest.Route{"PATCH", "/token/:token/environment", JobRestHandler(dispatcher, ApiPatchEnvironment)},
		rest.Route{"PUT", "/token/:token/linkcontainers", JobRestHandler(dispatcher, ApiLinkContainers)},
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

		if token.D == 0 {
			log.Println("http: Recommend passing 'd' as an argument for the current date")
		}
		if token.U == "" {
			log.Println("http: Recommend passing 'u' as an argument for the associated user")
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
	gearId, errg := NewIdentifier(token.ResourceLocator())
	if errg != nil {
		return nil, errg
	}
	if token.ResourceType() == "" {
		return nil, errors.New("A container must have an image identifier")
	}

	data := extendedCreateContainerData{}
	if r.Body != nil {
		dec := json.NewDecoder(limitedBodyReader(r))
		if err := dec.Decode(&data); err != nil && err != io.EOF {
			return nil, err
		}
	}
	if data.Ports == nil {
		data.Ports = make([]PortPair, 0)
	}

	return &createContainerJobRequest{
		NewHttpJobResponse(w.ResponseWriter, false),
		jobRequest{reqid},
		gearId,
		token.U,
		token.ResourceType(),
		&data,
	}, nil
}

func ApiListContainers(reqid RequestIdentifier, token *TokenData, w *rest.ResponseWriter, r *rest.Request) (Job, error) {
	return &listContainersRequest{NewHttpJobResponse(w.ResponseWriter, false), jobRequest{reqid}}, nil
}

func ApiListImages(reqid RequestIdentifier, token *TokenData, w *rest.ResponseWriter, r *rest.Request) (Job, error) {
	return &listImagesRequest{NewHttpJobResponse(w.ResponseWriter, false), jobRequest{reqid}}, nil
}

func ApiGetContainerLog(reqid RequestIdentifier, token *TokenData, w *rest.ResponseWriter, r *rest.Request) (Job, error) {
	gearId, errg := NewIdentifier(token.ResourceLocator())
	if errg != nil {
		return nil, errg
	}
	return &containerLogJobRequest{
		NewHttpJobResponse(w.ResponseWriter, false),
		jobRequest{reqid},
		gearId,
		token.U,
	}, nil
}

func ApiPutKeys(reqid RequestIdentifier, token *TokenData, w *rest.ResponseWriter, r *rest.Request) (Job, error) {
	data := extendedCreateKeysData{}
	if r.Body != nil {
		dec := json.NewDecoder(limitedBodyReader(r))
		if err := dec.Decode(&data); err != nil && err != io.EOF {
			return nil, err
		}
	}
	if err := data.Check(); err != nil {
		return nil, err
	}
	return &createKeysJobRequest{
		NewHttpJobResponse(w.ResponseWriter, true),
		jobRequest{reqid},
		token.U,
		&data,
	}, nil
}

func ApiPutRepository(reqid RequestIdentifier, token *TokenData, w *rest.ResponseWriter, r *rest.Request) (Job, error) {
	repositoryId, errg := NewIdentifier(token.ResourceLocator())
	if errg != nil {
		return nil, errg
	}
	// TODO: convert token into a safe clone spec and commit hash
	return &createRepositoryJobRequest{
		NewHttpJobResponse(w.ResponseWriter, false),
		jobRequest{reqid},
		repositoryId,
		token.U,
		"ccoleman/githost",
		token.ResourceType(),
	}, nil
}

func ApiPutContainerAction(reqid RequestIdentifier, token *TokenData, w *rest.ResponseWriter, r *rest.Request) (Job, error) {
	action := r.PathParam("action")
	gearId, errg := NewIdentifier(token.ResourceLocator())
	if errg != nil {
		return nil, errg
	}
	switch action {
	case "started":
		return &startedContainerStateJobRequest{
			NewHttpJobResponse(w.ResponseWriter, false),
			jobRequest{reqid},
			gearId,
			token.U,
		}, nil
	case "stopped":
		return &stoppedContainerStateJobRequest{
			NewHttpJobResponse(w.ResponseWriter, false),
			jobRequest{reqid},
			gearId,
			token.U,
		}, nil
	default:
		return nil, errors.New("You must provide a valid action for this container to take")
	}
}

func ApiPutBuildImageAction(reqid RequestIdentifier, token *TokenData, w *rest.ResponseWriter, r *rest.Request) (Job, error) {
	if token.ResourceLocator() == "" {
		return nil, errors.New("You must specifiy the application source to build")
	}
	if token.ResourceType() == "" {
		return nil, errors.New("You must specify a base image")
	}

	source := token.ResourceLocator() // token.R
	baseImage := token.ResourceType() // token.T
	tag := token.U

	data := extendedBuildImageData{}
	if r.Body != nil {
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&data); err != nil && err != io.EOF {
			return nil, err
		}
	}

	return &buildImageJobRequest{
		NewHttpJobResponse(w.ResponseWriter, false),
		jobRequest{reqid},
		source,
		baseImage,
		tag,
		&data,
	}, nil
}

func ApiPutEnvironment(reqid RequestIdentifier, token *TokenData, w *rest.ResponseWriter, r *rest.Request) (Job, error) {
	id, errg := NewIdentifier(token.ResourceLocator())
	if errg != nil {
		return nil, errg
	}

	data := extendedEnvironmentData{}
	if r.Body != nil {
		dec := json.NewDecoder(limitedBodyReader(r))
		if err := dec.Decode(&data); err != nil && err != io.EOF {
			return nil, err
		}
	}
	if err := data.Check(); err != nil {
		return nil, err
	}

	var source *url.URL
	if data.Source != "" {
		url, erru := url.Parse(data.Source)
		if erru != nil {
			return nil, erru
		}
		source = url
	}

	return &putEnvironmentJobRequest{
		NewHttpJobResponse(w.ResponseWriter, false),
		jobRequest{reqid},
		id,
		data.Env,
		source,
	}, nil
}

func ApiPatchEnvironment(reqid RequestIdentifier, token *TokenData, w *rest.ResponseWriter, r *rest.Request) (Job, error) {
	id, errg := NewIdentifier(token.ResourceLocator())
	if errg != nil {
		return nil, errg
	}

	data := extendedEnvironmentData{}
	if r.Body != nil {
		dec := json.NewDecoder(limitedBodyReader(r))
		if err := dec.Decode(&data); err != nil && err != io.EOF {
			return nil, err
		}
	}
	if err := data.Check(); err != nil {
		return nil, err
	}

	return &patchEnvironmentJobRequest{
		NewHttpJobResponse(w.ResponseWriter, false),
		jobRequest{reqid},
		id,
		data.Env,
	}, nil
}

func ApiGetContent(reqid RequestIdentifier, token *TokenData, w *rest.ResponseWriter, r *rest.Request) (Job, error) {
	if token.ResourceLocator() == "" {
		return nil, errors.New("You must specify the location of the content you want to access")
	}
	if token.ResourceType() == "" {
		return nil, errors.New("You must specify the type of the content you want to access")
	}

	return &contentJobRequest{
		NewHttpJobResponse(w.ResponseWriter, false),
		jobRequest{reqid},
		token.ResourceType(),
		token.ResourceLocator(),
		r.PathParam("*"),
	}, nil
}

func ApiLinkContainers(reqid RequestIdentifier, token *TokenData, w *rest.ResponseWriter, r *rest.Request) (Job, error) {
	id, errg := NewIdentifier(token.ResourceLocator())
	if errg != nil {
		return nil, errg
	}

	data := extendedLinkContainersData{}
	if r.Body != nil {
		dec := json.NewDecoder(limitedBodyReader(r))
		if err := dec.Decode(&data); err != nil && err != io.EOF {
			return nil, err
		}
	}

	return &linkContainersJobRequest{
		NewHttpJobResponse(w.ResponseWriter, false),
		jobRequest{reqid},
		id,
		&data,
	}, nil
}

func limitedBodyReader(r *rest.Request) io.Reader {
	return io.LimitReader(r.Body, 100*1024)
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

	if token.I == "" {
		i := make(RequestIdentifier, 16)
		_, errr := rand.Read(i)
		if errr != nil {
			rerr = &apiRequestError{errr, "Unable to generate token for this request: " + errr.Error(), http.StatusBadRequest}
			return
		}
		id = i
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
