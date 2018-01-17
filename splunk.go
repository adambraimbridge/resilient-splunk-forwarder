package main

import (
	"crypto/tls"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cyberdelia/go-metrics-graphite"
	"github.com/rcrowley/go-metrics"
	"github.com/sirupsen/logrus"
)

const (
	graphitePrefix  = "coco.services"
	graphitePostfix = "resilient-splunk-forwarder"
)

var (
	request_count   metrics.Counter
	error_count     metrics.Counter
	discarded_count metrics.Counter
)

type Forwarder interface {
	forward(s string, callback func(string, error))
}

type splunkClient struct {
	config appConfig
	client *http.Client
}

func NewSplunkForwarder(config appConfig) Forwarder {
	initMetrics(config)
	tlsConfig := &tls.Config{InsecureSkipVerify: true}
	transport := &http.Transport{
		TLSClientConfig:     tlsConfig,
		MaxIdleConnsPerHost: config.workers,
	}
	client := &http.Client{Transport: transport}

	return &splunkClient{client: client, config: config}
}

func (splunk *splunkClient) forward(s string, callback func(string, error)) {
	t := metrics.GetOrRegisterTimer("post.time", metrics.DefaultRegistry)
	t.Time(func() {
		req, err := http.NewRequest("POST", splunk.config.fwdURL, strings.NewReader(s))
		if err != nil {
			logrus.Println(err)
		}
		tokenWithKeyword := strings.Join([]string{"Splunk", splunk.config.token}, " ") //join strings "Splunk" and value of -token argument
		req.Header.Set("Authorization", tokenWithKeyword)
		request_count.Inc(1)
		r, err := splunk.client.Do(req)
		if err != nil {
			error_count.Inc(1)
			logrus.Println(err)
		} else {
			defer r.Body.Close()
			io.Copy(ioutil.Discard, r.Body)
			if r.StatusCode != 200 {
				error_count.Inc(1)
				logrus.Printf("Unexpected status code %v (%v) when sending %v to %v\n", r.StatusCode, r.Status, s, splunk.config.fwdURL)
				if r.StatusCode != 400 {
					err = errors.New(r.Status)
				} else {
					discarded_count.Inc(1)
					logrus.Printf("Discarding malformed message\n")
				}
			}
		}
		callback(s, err)
	})

}

func initMetrics(config appConfig) {
	graphiteNamespace := strings.Join([]string{graphitePrefix, config.env, graphitePostfix, config.hostname}, ".")
	// graphiteNamespace ~ prefix.env.postfix.hostname
	logrus.Printf("%v namespace: %v\n", config.graphiteServer, graphiteNamespace)
	if config.dryrun {
		logrus.Printf("Dryrun enabled, not connecting to %v\n", config.graphiteServer)
	} else {
		addr, err := net.ResolveTCPAddr("tcp", config.graphiteServer)
		if err != nil {
			logrus.Println(err)
		}
		go graphite.Graphite(metrics.DefaultRegistry, 5*time.Second, graphiteNamespace, addr)
	}
	go metrics.Log(metrics.DefaultRegistry, 5*time.Second, log.New(os.Stdout, "metrics ", log.Lmicroseconds))
	splunkMetrics()
}

func splunkMetrics() {
	request_count = metrics.GetOrRegisterCounter("splunk_requests_total", metrics.DefaultRegistry)
	error_count = metrics.GetOrRegisterCounter("splunk_requests_error", metrics.DefaultRegistry)
	discarded_count = metrics.GetOrRegisterCounter("splunk_requests_discarded", metrics.DefaultRegistry)
}
