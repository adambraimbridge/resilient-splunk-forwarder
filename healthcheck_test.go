package main

import (
	//"errors"
	"github.com/Financial-Times/service-status-go/httphandlers"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
	health "github.com/Financial-Times/go-fthealth/v1_1"
	"errors"
)

func TestGTGUnhealthyCluster(t *testing.T) {
	//create a request to pass to our handler
	req := httptest.NewRequest("GET", "/__gtg", nil)

	testCheck := []health.Check{
		{
			BusinessImpact:   "Logs are not reaching Splunk therefore monitoring may be affected",
			Name:             "Splunk healthcheck",
			PanicGuide:       "https://dewey.ft.com/resilient-splunk-forwarder.html",
			Severity:         1,
			TechnicalSummary: "Latest request to Splunk HEC has returned an error - check journal file",
			Checker: func() (string, error) {
				err := errors.New("test error")
				return "Splunk is not healthy", err
			},
		},
	}

	healthService := newHealthService(nil, testCheck)

	//create a responseRecorder
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(httphandlers.NewGoodToGoHandler(healthService.GTG))

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)
	actual := rr.Result()

	// Series of verifications:
	assert.Equal(t, http.StatusServiceUnavailable, actual.StatusCode, "status code")
	assert.Equal(t, "no-cache", actual.Header.Get("Cache-Control"), "cache-control header")
	assert.Equal(t, "test error", rr.Body.String(), "GTG response body")
}

func TestGTGHealthyCluster(t *testing.T) {
	//create a request to pass to our handler
	req := httptest.NewRequest("GET", "/__gtg", nil)

	testCheck := []health.Check{
		{
			BusinessImpact:   "Logs are not reaching Splunk therefore monitoring may be affected",
			Name:             "Splunk healthcheck",
			PanicGuide:       "https://dewey.ft.com/resilient-splunk-forwarder.html",
			Severity:         1,
			TechnicalSummary: "Latest request to Splunk HEC has returned an error - check journal file",
			Checker: func() (string, error) {
				return "Splunk is healthy", nil
			},
		},
	}

	healthService := newHealthService(nil, testCheck)

	//create a responseRecorder
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(httphandlers.NewGoodToGoHandler(healthService.GTG))

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)
	actual := rr.Result()

	// Series of verifications:
	assert.Equal(t, http.StatusOK, actual.StatusCode, "status code")
	assert.Equal(t, "no-cache", actual.Header.Get("Cache-Control"), "cache-control header")
	assert.Equal(t, "OK", rr.Body.String(), "GTG response body")
}
