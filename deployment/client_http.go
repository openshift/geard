package deployment

import (
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"time"
)

type HttpClient struct {
	client *http.Client
}

// Create new Http Client for use by deployment library
func NewHttpClient(insecure bool, timeout time.Duration) *HttpClient {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure},
		Dial:            timeoutDialer(timeout, timeout),
	}

	return &HttpClient{client: &http.Client{Transport: transport}}
}

func (h *HttpClient) Get(url string, headers map[string]string) (io.ReadCloser, error) {
	request, err := http.NewRequest("GET", url, nil)
	if nil != err {
		return nil, err
	}

	for key, value := range headers {
		request.Header.Add(key, value)
	}

	response, err := h.client.Do(request)
	if err != nil {
		return nil, err
	}

	if response.StatusCode == http.StatusOK {
		return response.Body, nil
	}

	return nil, errors.New("Get(" + url + "): " + response.Status)
}

func timeoutDialer(cTimeoutSeconds time.Duration, rwTimeoutSeconds time.Duration) func(net, addr string) (c net.Conn, err error) {
	return func(netw, addr string) (net.Conn, error) {
		conn, err := net.DialTimeout(netw, addr, (cTimeoutSeconds * time.Second))
		if err != nil {
			return nil, err
		}
		conn.SetDeadline(time.Now().Add(rwTimeoutSeconds * time.Second))
		return conn, nil
	}
}
