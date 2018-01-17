package main

import (
	health "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/service-status-go/gtg"
)

const healthPath = "/__health"

type healthService struct {
	config *healthConfig
	checks []health.Check
}

type healthConfig struct {
	appSystemCode string
	appName       string
	port          string
}

func newHealthService(config *healthConfig) *healthService {
	service := &healthService{config: config}
	service.checks = []health.Check{
		service.sampleCheck(),
	}
	return service
}

func (service *healthService) sampleCheck() health.Check {
	return health.Check{
		BusinessImpact:   "Sample healthcheck has no impact",
		Name:             "Sample healthcheck",
		PanicGuide:       "https://dewey.ft.com/resilient-splunk-forwarder.html",
		Severity:         1,
		TechnicalSummary: "Sample healthcheck has no technical details",
		Checker:          service.sampleChecker,
	}
}

func (service *healthService) sampleChecker() (string, error) {
	return "Sample is healthy", nil

}

func (service *healthService) GTG() gtg.Status {
	gtgChecks := []gtg.StatusChecker{}

	for _, check := range service.checks {
		gtgCheckFunc := func() gtg.Status {
			return gtgCheck(check.Checker)
		}
		gtgChecks = append(gtgChecks, gtgCheckFunc)
	}

	return gtg.FailFastParallelCheck(gtgChecks)()
}

func gtgCheck(handler func() (string, error)) gtg.Status {
	if _, err := handler(); err != nil {
		return gtg.Status{GoodToGo: false, Message: err.Error()}
	}
	return gtg.Status{GoodToGo: true}
}
