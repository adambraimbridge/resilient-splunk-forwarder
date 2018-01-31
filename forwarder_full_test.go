package main

import (
	"net/http"
	"net/http/httptest"

	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var config appConfig

const messageCount = 100

type splunkMock struct {
	sync.RWMutex
	index      []string
	errorCount int
}

func (splunk *splunkMock) append(s string) {
	splunk.Lock()
	defer splunk.Unlock()
	splunk.index = append(splunk.index, s)
}

func (splunk *splunkMock) getIndex() []string {
	splunk.Lock()
	defer splunk.Unlock()
	return splunk.index
}

func (splunk *splunkMock) incErrors() {
	splunk.Lock()
	defer splunk.Unlock()
	splunk.errorCount++
}

func (splunk *splunkMock) getErrorCount() int {
	splunk.Lock()
	defer splunk.Unlock()
	return splunk.errorCount
}

type s3ServiceMock struct {
	sync.RWMutex
	cache []string
}

var splunk = splunkMock{}

func (s3 *s3ServiceMock) ListAndDelete() ([]string, error) {
	s3.Lock()
	items := s3.cache
	s3.cache = make([]string, 0)
	s3.Unlock()
	return items, nil
}

func (s3 *s3ServiceMock) Put(obj string) error {
	obj = strings.Replace(obj, "retry", "safe", -1)
	obj = strings.Replace(obj, "error", "retry", -1)
	s3.Lock()
	s3.cache = append(s3.cache, obj)
	s3.Unlock()
	return nil
}

func (s3 *s3ServiceMock) getHealth() error {
	return nil
}

func TestMain(m *testing.M) {

	splunkTestServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bytes := make([]byte, r.ContentLength)
		r.Body.Read(bytes)
		defer r.Body.Close()
		body := string(bytes)
		if strings.Contains(body, "simulated_retry") {
			splunk.incErrors()
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			splunk.append(body)
			w.WriteHeader(http.StatusOK)
		}
	}))

	defer splunkTestServer.Close()

	graphiteTestServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	defer graphiteTestServer.Close()

	config = appConfig{}
	config.fwdURL = splunkTestServer.URL
	config.env = "dummy"
	graphiteURL, _ := url.Parse(graphiteTestServer.URL)
	config.graphiteServer = fmt.Sprintf("%v:%v", graphiteURL.Hostname(), graphiteURL.Port())
	config.workers = 8
	config.chanBuffer = 256
	config.token = "secret"
	config.bucket = "testbucket"

	os.Exit(m.Run())
}

func Test_Forwarder(t *testing.T) {

	s3 := &s3ServiceMock{}
	splunkForwarder := NewSplunkForwarder(config)
	logProcessor := NewLogProcessor(splunkForwarder, s3, config)
	go func() {
		logProcessor.Start()
	}()

	for i := 0; i < messageCount; i++ {
		if i == messageCount/2 {
			s3.Put(`{event:"simulated_error"}`)
		} else {
			s3.Put(`{event:"127.0.0.1 - - [21/Apr/2015:12:15:34 +0000] \"GET /eom-file/all/e09b49d6-e1fa-11e4-bb7f-00144feab7de HTTP/1.1\" 200 53706 919 919"}`)
		}
	}

	time.Sleep(3 * time.Second)

	assert.Equal(t, messageCount, len(splunk.getIndex()))
	assert.Equal(t, 1, splunk.getErrorCount())
	assert.Contains(t, strings.Join(splunk.getIndex(), ""), "simulated_safe")
}
