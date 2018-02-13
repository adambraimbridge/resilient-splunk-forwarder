package main

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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

func Test_Forwarder(t *testing.T) {
	s3 := &s3ServiceMock{}
	splunkForwarder := NewSplunkForwarder(mainConfig)
	logProcessor := NewLogProcessor(splunkForwarder, s3, mainConfig)
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

	assert.Equal(t, messageCount, len(splunk.getIndex()))
	assert.Equal(t, 1, splunk.getErrorCount())
	assert.Equal(t, nil, splunkForwarder.getHealth())
	assert.Contains(t, strings.Join(splunk.getIndex(), ""), "simulated_safe")
}
