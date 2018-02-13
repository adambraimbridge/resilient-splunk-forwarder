package main

import (
	"testing"
	"net/http/httptest"
	"strings"
	"net/url"
	"fmt"
	"os"
	"net/http"
	"os/exec"
)

var config appConfig

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

	os.Setenv("TOKEN", config.token)
	os.Setenv("BUCKET_NAME", config.bucket)
	os.Setenv("AWS_REGION", "eu")
	os.Setenv("AWS_ACCESS_KEY_ID", "accessKey")
	os.Setenv("FORWARDER_URL", config.fwdURL)

	app := initApp()

	go func() {
		app.Run(os.Args)
	}()

	os.Exit(m.Run())
}

func Test_successValidateParams(t *testing.T) {
	validationConfig := config
	validationConfig.fwdURL = "test-fwdURL"
	validationConfig.awsSecretKey = "awsSecretKey"
	validationConfig.awsAccessKey = "awsAccessKey"

	validateParams(validationConfig)
}

func Test_failValidateParams(t *testing.T) {

	if os.Getenv("INVALID_CONFIG") == "1" {
		validateParams(config)
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=Test_failValidateParams")
	cmd.Env = append(os.Environ(), "INVALID_CONFIG=1")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Fatalf("Process ran with err %v, want exit status 1", err)
}
