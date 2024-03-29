# smartlogic-notifier

[![Circle CI](https://circleci.com/gh/Financial-Times/smartlogic-notifier/tree/master.png?style=shield)](https://circleci.com/gh/Financial-Times/smartlogic-notifier/tree/master)[![Go Report Card](https://goreportcard.com/badge/github.com/Financial-Times/smartlogic-notifier)](https://goreportcard.com/report/github.com/Financial-Times/smartlogic-notifier) [![Coverage Status](https://coveralls.io/repos/github/Financial-Times/smartlogic-notifier/badge.svg)](https://coveralls.io/github/Financial-Times/smartlogic-notifier)

## Introduction

Entrypoint for concept publish notifications from the Smartlogic Semaphore system.

## Installation

Download the source code, dependencies and build the binary:

        go get github.com/Financial-Times/smartlogic-notifier
        cd $GOPATH/src/github.com/Financial-Times/smartlogic-notifier
        go install

To run the tests:

        go test -v -race ./...


2. Run the binary (using the `help` flag to see the available optional arguments):

        $GOPATH/bin/smartlogic-notifier [--help]

Options:

        --app-system-code="smartlogic-notifier"         System Code of the application ($APP_SYSTEM_CODE)
        --app-name="Smartlogic Notifier"                Application name ($APP_NAME)
        --kafkaAddresses="localhost:9092"               Comma separated list of Kafka broker addresses ($KAFKA_ADDRESSES)
        --kafkaTopic="SmartlogicConcept"                Kafka topic to send messages to ($KAFKA_TOPIC)
        --kafkaClusterArn=""                            Kafka cluster ARN used by the producer for maintenance monitoring ($KAFKA_CLUSTER_ARN)
        --smartlogicBaseURL=""                          Base URL for the Smartlogic instance ($SMARTLOGIC_BASE_URL)
        --smartlogicModel=""                            Smartlogic model to read from ($SMARTLOGIC_MODEL)
        --smartlogicAPIKey=""                           Smartlogic model to read from ($SMARTLOGIC_API_KEY)
        --smartlogicHealthcheckConcept=""               Concept uuid existing in the Smartlogic model to be used for healthcheck ($SMARTLOGIC_HEALTHCHECK_CONCEPT)
        --port="8080"                                   Port to listen on ($APP_PORT)
        --logLevel="info"                               Level of logging to be shown ($LOG_LEVEL)
        --healthcheckSuccessCacheTime="1m"              How long to cache a successful Smartlogic response for ($HEALTHCHECK_SUCCESS_CACHE_TIME)
        --conceptUriPrefix="http://www.ft.com/thing/"   The concept URI prefix to be added before the UUID part of the Smartlogic request path ($CONCEPT_URI_PREFIX)


## Build and deployment

* Built by Jenkins and uploaded to Docker Hub on merge to master: [coco/smartlogic-notifier](https://hub.docker.com/r/coco/smartlogic-notifier/)
* CI provided by CircleCI: [smartlogic-notifier](https://circleci.com/gh/Financial-Times/smartlogic-notifier)

## Service endpoints
Endpoints are documented in [Swagger](api.yml)

Based on the following [google doc](https://docs.google.com/document/d/1TeT9pM-f3Yo6oIBLyp4ZxgL8IR2y6LZU9n66yqD6DEE).


## Healthchecks
Admin endpoints are:

`/__gtg`

`/__health`

`/__build-info`

