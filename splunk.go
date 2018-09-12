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

	"github.com/prometheus/client_golang/prometheus"

	"sync"

	"github.com/cyberdelia/go-metrics-graphite"
	"github.com/rcrowley/go-metrics"
	"github.com/sirupsen/logrus"
)

const (
	graphitePrefix  = "coco.services"
	graphitePostfix = "resilient-splunk-forwarder"
)

var (
	request_count    metrics.Counter
	error_count      metrics.Counter
	discarded_count  metrics.Counter
	requestCounter   *prometheus.CounterVec
	errorCounter     *prometheus.CounterVec
	discardedCounter *prometheus.CounterVec
	envLabel         prometheus.Labels
)

type Forwarder interface {
	Healthy
	forward(s string, callback func(string, error))
}

type splunkClient struct {
	sync.Mutex
	config      appConfig
	client      *http.Client
	latestError error
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
		requestCounter.With(envLabel).Inc()
		r, err := splunk.client.Do(req)
		if err != nil {
			error_count.Inc(1)
			errorCounter.With(envLabel).Inc()
			logrus.Println(err)
		} else {
			defer r.Body.Close()
			io.Copy(ioutil.Discard, r.Body)
			if r.StatusCode != 200 {
				error_count.Inc(1)
				errorCounter.With(envLabel).Inc()
				logrus.Printf("Unexpected status code %v (%v) when sending %v to %v\n", r.StatusCode, r.Status, s, splunk.config.fwdURL)
				if r.StatusCode != 400 {
					err = errors.New(r.Status)
				} else {
					discarded_count.Inc(1)
					discardedCounter.With(envLabel).Inc()
					logrus.Printf("Discarding malformed message\n")
				}
			}
		}
		splunk.setHealth(err)
		callback(s, err)
	})

}

func (splunk *splunkClient) getHealth() error {
	splunk.Lock()
	defer splunk.Unlock()
	return splunk.latestError
}

func (splunk *splunkClient) setHealth(err error) {
	splunk.Lock()
	splunk.latestError = err
	splunk.Unlock()
}

func initMetrics(config appConfig) {
	graphiteNamespace := strings.Join([]string{graphitePrefix, config.env, graphitePostfix}, ".")
	// graphiteNamespace ~ prefix.env.postfix.hostname
	logrus.Printf("%v namespace: %v\n", config.graphiteServer, graphiteNamespace)
	addr, err := net.ResolveTCPAddr("tcp", config.graphiteServer)
	if err != nil {
		logrus.Println(err)
	}
	go graphite.Graphite(metrics.DefaultRegistry, 5*time.Second, graphiteNamespace, addr)
	go metrics.Log(metrics.DefaultRegistry, 5*time.Second, log.New(os.Stdout, "metrics ", log.Lmicroseconds))
	envLabel = prometheus.Labels{"environment": config.env}
	splunkMetrics()
}

func splunkMetrics() {

	requestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "upp",
			Subsystem: "splunk_forwarder",
			Name:      "request_count",
			Help:      "Number of requests to splunk",
		},
		[]string{
			"environment",
		})
	prometheus.MustRegister(requestCounter)

	errorCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "upp",
			Subsystem: "splunk_forwarder",
			Name:      "error_count",
			Help:      "Number of errors connecting to splunk",
		},
		[]string{
			"environment",
		})

	prometheus.MustRegister(errorCounter)

	discardedCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "upp",
			Subsystem: "splunk_forwarder",
			Name:      "discarded_count",
			Help:      "Number of discarded messages",
		},
		[]string{
			"environment",
		})
	prometheus.MustRegister(discardedCounter)

	request_count = metrics.GetOrRegisterCounter("splunk_requests_total", metrics.DefaultRegistry)
	error_count = metrics.GetOrRegisterCounter("splunk_requests_error", metrics.DefaultRegistry)
	discarded_count = metrics.GetOrRegisterCounter("splunk_requests_discarded", metrics.DefaultRegistry)
}
