package containers

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

func (e *Environment) FromString(s string) (bool, error) {
	pair := strings.SplitN(s, "=", 2)
	if len(pair) != 2 {
		return false, nil
	}
	second := pair[1]
	if strings.HasPrefix(second, "\"") || strings.HasPrefix(second, "'") {
		value, err := strconv.Unquote(second)
		if err != nil {
			return true, errors.New("The value for " + second + " is not valid: " + err.Error())
		}
		second = value
	}
	e.Name = pair[0]
	e.Value = second
	return true, nil
}

type EnvironmentVariables []Environment

func ExtractEnvironmentVariablesFrom(existing *[]string) (EnvironmentVariables, error) {
	args := *existing
	unchanged := make([]string, 0, len(args))
	variables := make(EnvironmentVariables, 0, 2)

	env := Environment{}
	for i := range args {
		arg := args[i]
		match, err := env.FromString(arg)
		if err != nil {
			return EnvironmentVariables{}, err
		}
		if match {
			variables = append(variables, env)
			env = Environment{}
		} else {
			unchanged = append(unchanged, arg)
		}
	}
	*existing = unchanged
	return variables, nil
}

type EnvironmentDescription struct {
	Variables []Environment
	Source    string
	Id        Identifier // Used on creation only
}

func (d *EnvironmentDescription) Empty() bool {
	if len(d.Variables) > 0 {
		return false
	}
	if d.Source != "" || d.Id != "" {
		return false
	}
	return true
}

func (d *EnvironmentDescription) Check() error {
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

// TODO: Return JobErrors that callers can react to
func (j *EnvironmentDescription) Fetch(upto int64) error {
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
		return errors.New("Unable to retrieve environment file from remote server.")
	}

	var r io.Reader = resp.Body
	if upto > 0 {
		r = &io.LimitedReader{r, upto}
	}
	if err := j.ReadFrom(r); err != nil {
		return err
	}
	return nil
}

// Write the provided enviroment data to an appropriate location
func (j *EnvironmentDescription) Write(appends bool) error {
	envPath := j.Id.EnvironmentPathFor()

	var file *os.File
	var err error

	if appends {
		file, err = os.OpenFile(envPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0660)
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

func (j *EnvironmentDescription) ReadFrom(r io.Reader) error {
	all := make(map[string]string)
	scanner := bufio.NewScanner(r)
	e := Environment{}
	for scanner.Scan() {
		s := scanner.Text()
		match, err := e.FromString(s)
		if err != nil {
			continue
		}
		if err := e.Check(); err != nil {
			continue
		}
		if match {
			all[e.Name] = e.Value
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	env := make(EnvironmentVariables, 0, len(all))
	for name := range all {
		e := Environment{name, all[name]}
		env = append(env, e)
	}
	j.Variables = env
	return nil
}

func (j *EnvironmentDescription) Map() map[string]string {
	env := make(map[string]string)
	vars := j.Variables

	for i := range vars {
		env[vars[i].Name] = vars[i].Value
	}

	return env
}
