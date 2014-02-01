package agent

import (
	"fmt"
	"log"
	"net/http"
)

func ServeApi(dispatcher *Dispatcher, w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.RequestURI)

	path, has := TakePrefix(r.RequestURI, "/token/")
	if !has {
		http.Error(w, "Token is required - pass /token/<token>/<path>", http.StatusForbidden)
		return
	}

	token, id, path, err := extractToken(path)
	if err != nil {
		http.Error(w, "Token is required - pass /token/<token>/<path>", http.StatusForbidden)
		return
	}

	if subpath, has := TakePrefix(path, "content/"); has || path == "content" {
		job, err := NewContentJob(id, token.ResourceType(), token.ResourceLocator(), subpath)
		if err != nil {
			serveRequestError(w, apiRequestError{err, "Content request is not properly formed: " + err.Error(), http.StatusBadRequest})
			return
		}

		fmt.Fprintf(w, "Job %+v (with token %+v to subpath %s)", job, token, subpath)
		dispatcher.Dispatch(job)
		return

	} else {
		switch path {
		case "container":
			if r.Method == "PUT" {

				job, err := NewCreateContainerJob(id, token.ResourceLocator(), token.ResourceType(), r.Body, w)
				if err != nil {
					serveRequestError(w, apiRequestError{err, "Content request is not properly formed: " + err.Error(), http.StatusBadRequest})
					return
				}

				fmt.Fprintf(w, "Job %+v (with token %+v to subpath %s)", job, token, subpath)
				dispatcher.Dispatch(job)
				return

			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
		}
	}
	http.NotFound(w, r)
}

func extractToken(path string) (token *TokenData, id []byte, subpath string, rerr *apiRequestError) {
	token, subpath, err := NewTokenFromPath(path)
	if err != nil {
		rerr = &apiRequestError{err, "Invalid authorization token", http.StatusForbidden}
		return
	}

	id, err = token.RequestId()
	if err != nil {
		rerr = &apiRequestError{err, "Token is missing data: " + err.Error(), http.StatusBadRequest}
		return
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
