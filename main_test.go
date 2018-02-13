package main

import (
	"testing"
	"os"
	"os/exec"
	"strings"
	"net/url"
	"fmt"
	"net/http/httptest"
	"net/http"
	"time"
)

var mainConfig = appConfig{}
var splunkTestServer *httptest.Server

func TestMain(m *testing.M) {
	splunkTestServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	mainConfig.fwdURL = splunkTestServer.URL
	mainConfig.env = "dummy"
	mainConfig.port = "8081"
	graphiteURL, _ := url.Parse(graphiteTestServer.URL)
	mainConfig.graphiteServer = fmt.Sprintf("%v:%v", graphiteURL.Hostname(), graphiteURL.Port())
	mainConfig.workers = 8
	mainConfig.chanBuffer = 256
	mainConfig.token = "secret"
	mainConfig.bucket = "testbucket"
	mainConfig.awsRegion = "test-region"
	mainConfig.awsAccessKey = "test-access-key"
	mainConfig.awsSecretKey = "test-secret-key"

	os.Setenv("TOKEN", mainConfig.token)
	os.Setenv("BUCKET_NAME", mainConfig.bucket)
	os.Setenv("AWS_REGION", mainConfig.awsRegion)
	os.Setenv("AWS_ACCESS_KEY_ID", mainConfig.awsAccessKey)
	os.Setenv("AWS_REGION", mainConfig.awsRegion)
	os.Setenv("FORWARDER_URL", mainConfig.fwdURL)

	app := initApp()

	go func() {
		app.Run(os.Args)
	}()

	os.Exit(m.Run())
}

func Test_successValidateParams(t *testing.T) {
	mainConfig.fwdURL = "test-fwdURL"

	validateParams(mainConfig)
}

func Test_failValidateParams(t *testing.T) {
	mainConfig.fwdURL = ""

	if os.Getenv("INVALID_CONFIG") == "1" {
		validateParams(mainConfig)
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

func waitForService() {
	client := &http.Client{}
	retryCount := 0
	for {
		retryCount++
		if retryCount > 5 {
			fmt.Printf("Unable to start server")
			os.Exit(-1)
		}
		req, _ := http.NewRequest("GET", "http://localhost:8080/__gtg", nil)
		res, err := client.Do(req)
		if err == nil && res.StatusCode == http.StatusOK {
			break
		}
		time.Sleep(1000 * time.Millisecond)
	}
}
