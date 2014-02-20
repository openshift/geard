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
	Variables []Environment
	Source    string
	Id        Identifier // Used on creation only
}

func (d *extendedEnvironmentData) Check() error {
	for i := range d.Variables {
		e := &d.Variables[i]
		if err := e.Check(); err != nil {
			return err
		}
	}
	if d.Source != "" {
		_, erru := url.Parse(d.Source)
		if erru != nil {
			return erru
		}
	}
	return nil
}

type putEnvironmentJobRequest struct {
	JobResponse
	jobRequest
	*extendedEnvironmentData
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
	if j.Source != "" {
		if err := j.Fetch(); err != nil {
			j.Failure(ErrEnvironmentUpdateFailed)
			return
		}
	}
	if err := j.Write(false); err != nil {
		j.Failure(ErrEnvironmentUpdateFailed)
		return
	}

	j.Success(JobResponseOk)
}

type patchEnvironmentJobRequest struct {
	JobResponse
	jobRequest
	*extendedEnvironmentData
}

func (j *patchEnvironmentJobRequest) Execute() {
	if err := j.Write(true); err != nil {
		j.Failure(ErrEnvironmentUpdateFailed)
		return
	}
	j.Success(JobResponseOk)
}

// TODO: Return JobErrors that callers can react to
func (j *extendedEnvironmentData) Fetch() error {
	if j.Source == "" {
		return nil
	}

	var client http.Client
	resp, err := client.Get(j.Source)
	if err != nil {
		log.Print("job_environment: Unable to load the environment file from", j.Source, ":", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("job_environment: fetch status code is %d from %s", resp.StatusCode, j.Source)
		return err
	}

	env := j.Variables
	scanner := NewEnvironmentScanner(&io.LimitedReader{resp.Body, 100 * 1024})
	for scanner.Scan() {
		name, value := scanner.Environment()
		e := Environment{name, value}
		if erre := e.Check(); erre != nil {
			log.Print("job_environment: One of the environment variables was invalid: ", erre)
			return err
		}
		env = append(env, e)
	}
	if errs := scanner.Err(); errs != nil {
		log.Printf("job_environment: error reading environment from source %s: %v", j.Source, errs)
		return err
	}

	j.Variables = env
	return nil
}

// Write the provided enviroment data to an appropriate location
func (j *extendedEnvironmentData) Write(appends bool) error {
	envPath := j.Id.EnvironmentPathFor()

	var file *os.File
	var err error

	if appends {
		file, err = os.OpenFile(envPath, os.O_APPEND|os.O_WRONLY, 0660)
	} else {
		file, err = os.OpenFile(envPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0660)
		if os.IsExist(err) {
			file, err = os.OpenFile(envPath, os.O_TRUNC|os.O_WRONLY, 0660)
		}
	}
	if err != nil {
		log.Print("job_environment: Unable to open environment file: ", err)
		return err
	}
	defer file.Close()

	env := j.Variables
	for i := range env {
		if _, errw := fmt.Fprintf(file, "%s=%s\n", env[i].Name, strconv.Quote(env[i].Value)); errw != nil {
			log.Print("job_environment: Unable to write to environment file: ", err)
			return err
		}
	}
	if errc := file.Close(); errc != nil {
		log.Print("job_environment: Unable to close environment file: ", errc)
		return err
	}
	return nil
}
