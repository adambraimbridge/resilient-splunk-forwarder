package main

import (
	"github.com/pkg/errors"
	"net/http"
	"testing"
	"time"
)

type splunkClientMock struct {
	Forwarder
	config      appConfig
	client      *http.Client
	latestError error
}

func (splunk *splunkClientMock) forward(s string, callback func(string, error)) {
	if s == `{event:"127.0.0.1 - - [21/Apr/2015:12:15:34 +0000] \"GET /eom-file/all/e09b49d6-e1fa-11e4-bb7f-00144feab7de HTTP/1.1\" 200 53706 919 919"}` {
		callback("test", nil)
	} else if s == `{event:"simulated_retry"}` {
		callback("test", errors.New("test-error"))
	}
}

func Test_Processor(t *testing.T) {
	s3 := &s3ServiceMock{}
	splunkForwarderMock := &splunkClientMock{}
	logProcessor := NewLogProcessor(splunkForwarderMock, s3, config)
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

	go func() {
		logProcessor.Stop()
	}()
}
