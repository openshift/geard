package geard

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Environment struct {
	Name  string
	Value string
}

func (e *Environment) Check() error {
	if e.Name == "" || strings.TrimSpace(e.Name) == "" {
		return errors.New("Name may not be empty")
	}
	if len(e.Name) > 1024 {
		return errors.New("Name must be shorter than 1024 characters.")
	}
	if len(e.Value) > 8*1024 {
		return errors.New("Value must be less than 8KB.")
	}
	return nil
}

type extendedEnvironmentData struct {
	Env    []Environment
	Source string
}

func (d *extendedEnvironmentData) Check() error {
	for i := range d.Env {
		e := &d.Env[i]
		if err := e.Check(); err != nil {
			return err
		}
	}
	return nil
}

type putEnvironmentJobRequest struct {
	JobResponse
	jobRequest
	EnvId  Identifier
	Env    []Environment
	Source *url.URL
}

type EnvironmentScanner interface {
	Scan() bool
	Err() error
	Environment() (string, string)
}

type environmentScanner struct {
	*bufio.Scanner
	err error
}

func (s environmentScanner) Environment() (string, string) {
	token := s.Text()
	pair := strings.SplitN(token, "=", 2)
	value, err := strconv.Unquote(pair[1])
	if err != nil {
		value = ""
		s.err = err
	}
	return pair[0], value
}

func (s environmentScanner) Scan() bool {
	if s.err != nil {
		return false
	}
	return s.Scanner.Scan()
}

func (s environmentScanner) Err() error {
	if s.err != nil {
		return s.err
	}
	return s.Scanner.Err()
}

func NewEnvironmentScanner(r io.Reader) EnvironmentScanner {
	return environmentScanner{Scanner: bufio.NewScanner(r)}
}

func (j *putEnvironmentJobRequest) Execute() {
	env := j.Env
	if j.Source != nil {
		var client http.Client
		resp, err := client.Get(j.Source.String())
		if err != nil {
			log.Print("job_environment: Unable to load the environment file from", j.Source, ":", err)
			j.Failure(ErrEnvironmentUpdateFailed)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			log.Printf("job_environment: fetch status code is %d from %s", resp.StatusCode, j.Source)
			j.Failure(ErrEnvironmentUpdateFailed)
			return
		}
		scanner := NewEnvironmentScanner(&io.LimitedReader{resp.Body, 100 * 1024})
		for scanner.Scan() {
			name, value := scanner.Environment()
			e := Environment{name, value}
			if erre := e.Check(); erre != nil {
				log.Print("job_environment: One of the environment variables was invalid: ", erre)
				j.Failure(ErrEnvironmentUpdateFailed)
				return
			}
			env = append(env, e)
		}
		if errs := scanner.Err(); errs != nil {
			log.Printf("job_environment: error reading environment from source %s: %v", j.Source, errs)
			j.Failure(ErrEnvironmentUpdateFailed)
			return
		}
		resp.Body.Close()
	}

	envPath := j.EnvId.EnvironmentPathFor()
	file, err := os.OpenFile(envPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0660)
	if os.IsExist(err) {
		file, err = os.OpenFile(envPath, os.O_TRUNC|os.O_WRONLY, 0660)
	}
	if err != nil {
		log.Print("job_environment: Unable to create environment file: ", err)
		j.Failure(ErrEnvironmentUpdateFailed)
		return
	}
	defer file.Close()

	for i := range env {
		if _, errw := fmt.Fprintf(file, "%s=%s\n", env[i].Name, strconv.Quote(env[i].Value)); errw != nil {
			log.Print("job_environment: Unable to write to environment file: ", err)
			j.Failure(ErrEnvironmentUpdateFailed)
			return
		}
	}
	if errc := file.Close(); errc != nil {
		log.Print("job_environment: Unable to close environment file: ", errc)
		j.Failure(ErrEnvironmentUpdateFailed)
		return
	}
	j.Success(JobResponseOk)
}

type patchEnvironmentJobRequest struct {
	JobResponse
	jobRequest
	EnvId Identifier
	Env   []Environment
}

func (j *patchEnvironmentJobRequest) Execute() {
	env := j.Env

	envPath := j.EnvId.EnvironmentPathFor()
	file, err := os.OpenFile(envPath, os.O_APPEND|os.O_WRONLY, 0660)
	if err != nil {
		log.Print("job_environment: Unable to create environment file: ", err)
		j.Failure(ErrEnvironmentUpdateFailed)
		return
	}
	defer file.Close()

	for i := range env {
		if _, errw := fmt.Fprintf(file, "%s=%s\n", env[i].Name, strconv.Quote(env[i].Value)); errw != nil {
			log.Print("job_environment: Unable to write to environment file: ", err)
			j.Failure(ErrEnvironmentUpdateFailed)
			return
		}
	}
	if errc := file.Close(); errc != nil {
		j.Failure(ErrEnvironmentUpdateFailed)
		return
	}
	j.Success(JobResponseOk)
}
