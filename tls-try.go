package main

import (
	"crypto/tls"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
)

type TlsTryForwarder interface {
	forward(s string)
}

type TlsTrySplunkClient struct {
	client *http.Client
}

func NewTlsTrySplunkForwarder() TlsTryForwarder {
	//tlsConfig := &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12}
	tlsConfig := &tls.Config{InsecureSkipVerify: true}
	transport := &http.Transport{
		TLSClientConfig:     tlsConfig,
		MaxIdleConnsPerHost: 5,
	}
	client := &http.Client{Transport: transport}

	return &TlsTrySplunkClient{client: client}
}

func (splunk *TlsTrySplunkClient) forward(s string) {
	tls2_1_URL := "https://fancyssl.hboeck.de/"
	req, err := http.NewRequest("POST", tls2_1_URL, strings.NewReader(s))
	if err != nil {
		logrus.Println(err)
	}
	req.Header.Set("Authorization", "bla")
	r, err := splunk.client.Do(req)
	if err != nil {
		logrus.Println(err)
	} else {
		defer r.Body.Close()
		io.Copy(ioutil.Discard, r.Body)
		logrus.Printf("status code %v (%v) when sending %v to %v\n", r.StatusCode, r.Status, s, tls2_1_URL)
	}
}

func main() {
	tryTls2_1Forwarder := NewTlsTrySplunkForwarder()
	tryTls2_1Forwarder.forward("foobar")
}
