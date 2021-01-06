// Package client is fast http client
// copied from https://gist.github.com/AngerM/5059b3ff16292f3fbee990bae7046381
package client

import (
	"context"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/rs/dnscache"
)

var transporter *http.Transport

func init() {
	/*
		'High performance' http transport for golang
		increases MaxIdleConns and conns per host since we expect
		to be talking to a lot of other hosts all the time
		Also adds a basic in-process dns cache to help
		in docker environments since the standard alpine build appears
		to have no in container dns cache
	*/
	r := &dnscache.Resolver{}
	transporter = &http.Transport{
		DialContext: func(ctx context.Context, network string, addr string) (conn net.Conn, err error) {
			separator := strings.LastIndex(addr, ":")
			ips, err := r.LookupHost(ctx, addr[:separator])
			if err != nil {
				return nil, err
			}
			for _, ip := range ips {
				conn, err = net.Dial(network, ip+addr[separator:])
				if err == nil {
					break
				}
			}
			return
		},
		MaxIdleConns:    1024,
		MaxConnsPerHost: 100,
		IdleConnTimeout: 10 * time.Second,
	}
	go func() {
		clearUnused := true
		t := time.NewTicker(5 * time.Minute)
		defer t.Stop()
		for range t.C {
			r.Refresh(clearUnused)
		}
	}()
}

func getClient(timeout time.Duration) http.Client {
	return http.Client{
		Transport: transporter,
		Timeout:   timeout,
	}
}

func buildReq(method, url string, body io.Reader, headers http.Header) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err == nil {
		for header, vals := range headers {
			for _, val := range vals {
				req.Header.Add(header, val)
			}
		}
	}
	return req, err
}

// DoHTTPReq Wrapper func to use our transport and do a Get or Post request to the other side.
// Method is determined by whether you have a body or not
// Example:
// 	utils.DoHttpReq("https://www.google.com", 10 * time.Second, nil, nil)
// 	will send a Get request to Google and wait for up to 10 seconds
//
func DoHTTPReq(url, vhost string, timeout time.Duration, headers http.Header, body io.Reader) (*http.Response, error) {
	method := http.MethodGet
	if body != nil {
		method = http.MethodPost
	}
	req, err := buildReq(method, url, body, headers)
	if err != nil {
		return nil, err
	}
	if vhost != "" {
		req.Host = vhost
	}
	c := getClient(timeout)
	return c.Do(req)
}

// DoFireAndForgetHTTPReq Wrapper func to use our transport and do a Get or Post request to the other side.
// Method is determined by whether you have a body or not. This method does the request in
// the background and just drops the result
// Example:
// 	utils.DoFireAndForgetHTTPReq("https://www.google.com", 10 * time.Second, nil, nil)
// 	will send a Get request to Google and wait for up to 10 seconds
//
func DoFireAndForgetHTTPReq(url, vhost string, timeout time.Duration, headers http.Header, body io.Reader) {
	go func() {
		resp, _ := DoHTTPReq(url, vhost, timeout, headers, body)
		if resp != nil {
			// Consume the entire body so we can reuse this connection
			defer resp.Body.Close()
			ioutil.ReadAll(resp.Body)
		}
	}()
}
