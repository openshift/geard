package geard

import (
	"errors"
	//"fmt"
	"log"
	"net/http"
)

func ServeApi(dispatcher *Dispatcher, w http.ResponseWriter, r *http.Request) {
	path, has := TakePrefix(r.URL.Path, "/token/")
	if !has {
		http.Error(w, "Token is required - pass /token/<token>/<path>", http.StatusForbidden)
		return
	}

	token, id, path, errt := extractToken(path, r)
	if errt != nil {
		log.Println(errt)
		http.Error(w, "Token is required - pass /token/<token>/<path>", http.StatusForbidden)
		return
	}

	var job Job
	if subpath, has := TakePrefix(path, "content/"); has || path == "content" {
		j, err := NewContentJob(id, token.ResourceType(), token.ResourceLocator(), subpath, w)
		if err != nil {
			serveRequestError(w, apiRequestError{err, "Content request is not properly formed: " + err.Error(), http.StatusBadRequest})
			return
		}
		job = j

	} else {
		switch path {

		case "container":
			if r.Method != "PUT" {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}

			j, err := NewCreateContainerJob(id, token.ResourceLocator(), token.ResourceType(), r.Body, w)
			if err != nil {
				serveRequestError(w, apiRequestError{err, "Create container request is not properly formed: " + err.Error(), http.StatusBadRequest})
				return
			}
			job = j
		}
	}

	if job == nil {
		http.NotFound(w, r)
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

func extractToken(path string, r *http.Request) (token *TokenData, id RequestIdentifier, subpath string, rerr *apiRequestError) {
	segment, subpath, has := TakeSegment(path)
	if !has {
		rerr = &apiRequestError{errors.New("No matching token path"), "Invalid authorization token", http.StatusForbidden}
	}

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
