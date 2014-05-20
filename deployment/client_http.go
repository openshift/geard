package deployment

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// Create new Http Client for use by deployment library
func NewHttpClient(insecure bool, timeout time.Duration) *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure},
		Dial:            timeoutDialer(timeout, timeout),
	}

	return &http.Client{
		Transport: transport,
	}
}

func timeoutDialer(connectionTimeout time.Duration, readWriteDeadline time.Duration) func(net, addr string) (c net.Conn, err error) {
	return func(netw, addr string) (net.Conn, error) {
		conn, err := net.DialTimeout(netw, addr, (connectionTimeout * time.Second))
		if nil != err {
			return nil, err
		}

		return conn, conn.SetDeadline(time.Now().Add(readWriteDeadline * time.Second))
	}
}
