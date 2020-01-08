# resilient-splunk-forwarder

[![Circle CI](https://circleci.com/gh/Financial-Times/resilient-splunk-forwarder/tree/master.png?style=shield)](https://circleci.com/gh/Financial-Times/resilient-splunk-forwarder/tree/master)[![Go Report Card](https://goreportcard.com/badge/github.com/Financial-Times/resilient-splunk-forwarder)](https://goreportcard.com/report/github.com/Financial-Times/resilient-splunk-forwarder) [![Coverage Status](https://coveralls.io/repos/github/Financial-Times/resilient-splunk-forwarder/badge.svg)](https://coveralls.io/github/Financial-Times/resilient-splunk-forwarder)

## Introduction

Forwards logs cached in S3 to Splunk

## Installation

        go get -u github.com/Financial-Times/resilient-splunk-forwarder
        cd $GOPATH/src/github.com/Financial-Times/resilient-splunk-forwarder
        go build -mod=readonly .

## Running locally

1. Run the tests and install the binary:

    ` go test -mod=readonly -race ./... `
    ` ./resilent-splunk-forwarder `


2. Run the binary (using the `help` flag to see the available optional arguments):

        $GOPATH/bin/resilient-splunk-forwarder [--help]

Options:

          --app-system-code="resilient-splunk-forwarder"   System Code of the application ($APP_SYSTEM_CODE)
          --app-name="Resilient Splunk Forwarder"          Application name ($APP_NAME)
          --port="8080"                                    Port to listen on ($APP_PORT)
          --url=""                                         The url to forward to ($FORWARD_URL)
          --env="dummy"                                    environment_tag value ($ENV)
          --graphiteserver="graphite.ft.com:2003"          Graphite server host name and port ($GRAPHITE_SERVER)
          --workers=8                                      Number of concurrent workers ($WORKERS)
          --buffer=256                                     Channel buffer size ($CHAN_BUFFER)
          --token=""                                       Splunk HEC Authorization token ($TOKEN)
          --bucketName=""                                  S3 bucket for caching failed events ($BUCKET_NAME)
          --awsRegion=""                                   AWS region for S3 ($AWS_REGION)
          --awsAccessKey=""                                AWS Access Key for S3 ($AWS_ACCESS_KEY_ID)
          --awsSecretAccessKey=""                          AWS secret access key for S3 ($AWS_SECRET_ACCESS_KEY)

3. Test:

    The service reads and deletes objects from S3 and forwards them to the provided Splunk HEC URL, therefore local testing is not recommended.

## Build and deployment

* Built by Docker Hub on merge to master: [coco/resilient-splunk-forwarder](https://hub.docker.com/r/coco/resilient-splunk-forwarder/)
* CI provided by CircleCI: [resilient-splunk-forwarder](https://circleci.com/gh/Financial-Times/resilient-splunk-forwarder)

## Service endpoints

The app has no service endpoints.

## Healthchecks
Admin endpoints are:

`/__gtg`

`/__health`

`/__build-info`

There are several checks performed:

* Checks that the last S3 operation was successful
* Checks that the last Splunk operation was successful

Healthchecks incur no additional requests to external systems.

## Other information
There is a single thread listing objects from S3, but actual data is fetched asynchronously. Messages are immediately deleted from S3.
Messages are then dispatched to a set of workers that submit the data to the configured Splunk HEC URL.
Failed messages are stored again in S3. Failures also cause exponential backoff so that the endopint is not overwhelmed. 
However, due to having multiple workers, this will not affect messages that are already dispatched.  

### Logging

* The application uses [logrus](https://github.com/Sirupsen/logrus); the log file is initialised in [main.go](main.go).