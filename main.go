package main

import (
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jawher/mow.cli"
	"github.com/sirupsen/logrus"

	health "github.com/Financial-Times/go-fthealth/v1_1"
	status "github.com/Financial-Times/service-status-go/httphandlers"
)

const appDescription = "Forwards logs cached in S3 to Splunk"

type appConfig struct {
	appSystemCode  string
	appName        string
	port           string
	fwdURL         string
	env            string
	graphiteServer string
	dryrun         bool
	workers        int
	chanBuffer     int
	hostname       string
	token          string
	bucket         string
	awsRegion      string
}

func main() {
	app := cli.App("resilient-splunk-forwarder", appDescription)

	appSystemCode := app.String(cli.StringOpt{
		Name:   "app-system-code",
		Value:  "resilient-splunk-forwarder",
		Desc:   "System Code of the application",
		EnvVar: "APP_SYSTEM_CODE",
	})
	appName := app.String(cli.StringOpt{
		Name:   "app-name",
		Value:  "Resilient Splunk Forwarder",
		Desc:   "Application name",
		EnvVar: "APP_NAME",
	})
	port := app.String(cli.StringOpt{
		Name:   "port",
		Value:  "8080",
		Desc:   "Port to listen on",
		EnvVar: "APP_PORT",
	})
	fwdURL := app.String(cli.StringOpt{
		Name:   "url",
		Value:  "",
		Desc:   "The url to forward to",
		EnvVar: "FORWARD_URL",
	})
	env := app.String(cli.StringOpt{
		Name:   "env",
		Value:  "dummy",
		Desc:   "environment_tag value",
		EnvVar: "ENV",
	})
	graphiteServer := app.String(cli.StringOpt{
		Name:   "graphiteserver",
		Value:  "graphite.ft.com:2003",
		Desc:   "Graphite server host name and port",
		EnvVar: "GRAPHITE_SERVER",
	})
	dryrun := app.Bool(cli.BoolOpt{
		Name:   "dryrun",
		Value:  false,
		Desc:   "Dryrun true disables network connectivity. Use it for testing offline. Default value false",
		EnvVar: "DRYRUN",
	})
	workers := app.Int(cli.IntOpt{
		Name:   "workers",
		Value:  8,
		Desc:   "Number of concurrent workers",
		EnvVar: "WORKERS",
	})
	chanBuffer := app.Int(cli.IntOpt{
		Name:   "buffer",
		Value:  256,
		Desc:   "Channel buffer size",
		EnvVar: "CHAN_BUFFER",
	})
	hostname := app.String(cli.StringOpt{
		Name:   "hostname",
		Value:  "",
		Desc:   "Hostname running the service. If empty Go is trying to resolve the hostname.",
		EnvVar: "HOSTNAME",
	})
	token := app.String(cli.StringOpt{
		Name:   "token",
		Value:  "",
		Desc:   "Splunk HEC Authorization token",
		EnvVar: "TOKEN",
	})
	bucket := app.String(cli.StringOpt{
		Name:   "bucketName",
		Value:  "",
		Desc:   "S3 bucket for caching failed events",
		EnvVar: "BUCKET_NAME",
	})
	awsRegion := app.String(cli.StringOpt{
		Name:   "awsRegion",
		Value:  "",
		Desc:   "AWS region for S3",
		EnvVar: "AWS_REGION",
	})

	config := appConfig{
		appSystemCode:  *appSystemCode,
		appName:        *appName,
		port:           *port,
		fwdURL:         *fwdURL,
		env:            *env,
		graphiteServer: *graphiteServer,
		dryrun:         *dryrun,
		workers:        *workers,
		chanBuffer:     *chanBuffer,
		hostname:       *hostname,
		token:          *token,
		bucket:         *bucket,
		awsRegion:      *awsRegion,
	}

	logrus.SetLevel(logrus.InfoLevel)
	logrus.Infof("[Startup] resilient-splunk-forwarder is starting ")

	validateParams(config)

	app.Action = func() {
		logrus.Infof("System code: %s, App Name: %s, Port: %s", *appSystemCode, *appName, *port)
		defer logrus.Printf("Resilient Splunk forwarder: Stopped\n")

		s3, _ := NewS3Service(config.bucket, config.awsRegion, config.env)
		splunkForwarder := NewSplunkForwarder(config)
		logProcessor := NewLogProcessor(splunkForwarder, s3, config)

		logProcessor.Start()

		healthService := newHealthService(
			&healthConfig{appSystemCode: *appSystemCode, appName: *appName, port: *port},
			[]health.Check{
				{
					BusinessImpact:   "Logs are not reaching Splunk therefore monitoring may be affected",
					Name:             "Splunk healthcheck",
					PanicGuide:       "https://dewey.ft.com/resilient-splunk-forwarder.html",
					Severity:         1,
					TechnicalSummary: "Latest request to Splunk HEC has returned an error - check journal file",
					Checker: func() (string, error) {
						err := splunkForwarder.getHealth()
						if err != nil {
							return "Splunk is not healthy", err
						}
						return "Splunk is healthy", nil
					},
				},
				{
					BusinessImpact:   "Logs can not be read from S3 and will probably be indexed with delay",
					Name:             "S3 healthcheck",
					PanicGuide:       "https://dewey.ft.com/resilient-splunk-forwarder.html",
					Severity:         1,
					TechnicalSummary: "Latest request to S3 has returned an error - check journal file",
					Checker: func() (string, error) {
						err := splunkForwarder.getHealth()
						if err != nil {
							return "S3 is not healthy", err
						}
						return "S3 is healthy", nil
					},
				},
			},
		)
		go func() {
			serveEndpoints(healthService, *appSystemCode, *appName, *port)
		}()

		logrus.Printf("Resilient Splunk forwarder (workers %v): Started\n", workers)
		waitForSignal()
	}
	err := app.Run(os.Args)
	if err != nil {
		logrus.Errorf("App could not start, error=[%s]\n", err)
		return
	}
}

func serveEndpoints(healthService *healthService, appSystemCode string, appName string, port string) {

	serveMux := http.NewServeMux()

	hc := health.TimedHealthCheck{
		HealthCheck: health.HealthCheck{
			SystemCode: appSystemCode,
			Name:       appName, Description: appDescription,
			Checks: healthService.checks},
		Timeout: 10 * time.Second,
	}
	serveMux.HandleFunc(healthPath, health.Handler(hc))
	serveMux.HandleFunc(status.GTGPath, status.NewGoodToGoHandler(healthService.GTG))
	serveMux.HandleFunc(status.BuildInfoPath, status.BuildInfoHandler)

	server := &http.Server{Addr: ":" + port, Handler: serveMux}

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		if err := server.ListenAndServe(); err != nil {
			logrus.Infof("HTTP server closing with message: %v", err)
		}
		wg.Done()
	}()

	waitForSignal()
	logrus.Infof("[Shutdown] resilient-splunk-forwarder is shutting down")

	if err := server.Close(); err != nil {
		logrus.Errorf("Unable to stop http server: %v", err)
	}

	wg.Wait()
}

func waitForSignal() {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
}

func validateParams(config appConfig) {
	if len(config.fwdURL) == 0 { //Check whether -url parameter value was provided
		logrus.Printf("Forwarder URL must be provided\n")
		os.Exit(1) //If not fail visibly as we are unable to send logs to Splunk
	}
	if len(config.token) == 0 { //Check whether -token parameter value was provided
		logrus.Printf("Splunk token must be provided\n")
		os.Exit(1) //If not fail visibly as we are unable to send logs to Splunk
	}
	if len(config.hostname) == 0 { //Check whether -hostname parameter was provided. If not attempt to resolve
		hname, err := os.Hostname() //host name reported by the kernel, used for graphiteNamespace
		if err != nil {
			logrus.Println(err)
			hname = "unkownhost" //Set host name as unkownhost if hostname resolution fail
		}
		config.hostname = hname
	}
	if len(config.bucket) == 0 { //Check whether -bucket parameter value was provided
		logrus.Printf("S3 bucket name must be provided\n")
		os.Exit(1) //If not fail visibly as we are unable to send logs to Splunk
	}
}
