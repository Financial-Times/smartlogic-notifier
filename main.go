package main

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/kafka-client-go/v3"
	"github.com/Financial-Times/smartlogic-notifier/notifier"
	"github.com/Financial-Times/smartlogic-notifier/smartlogic"
	"github.com/gorilla/mux"
	cli "github.com/jawher/mow.cli"
	"github.com/sethgrid/pester"
)

const appDescription = "Entrypoint for concept publish notifications from the Smartlogic Semaphore system"

func main() {

	app := cli.App("smartlogic-notifier", appDescription)

	appSystemCode := app.String(cli.StringOpt{
		Name:   "app-system-code",
		Value:  "smartlogic-notifier",
		Desc:   "System Code of the application",
		EnvVar: "APP_SYSTEM_CODE",
	})

	appName := app.String(cli.StringOpt{
		Name:   "app-name",
		Value:  "Smartlogic Notifier",
		Desc:   "Application name",
		EnvVar: "APP_NAME",
	})

	kafkaAddresses := app.String(cli.StringOpt{
		Name:   "kafkaAddresses",
		Value:  "localhost:9092",
		Desc:   "Comma separated list of Kafka broker addresses",
		EnvVar: "KAFKA_ADDRESSES",
	})

	kafkaTopic := app.String(cli.StringOpt{
		Name:   "kafkaTopic",
		Value:  "SmartlogicConcept",
		Desc:   "Kafka topic to send messages to",
		EnvVar: "KAFKA_TOPIC",
	})

	smartlogicBaseURL := app.String(cli.StringOpt{
		Name:   "smartlogicBaseURL",
		Desc:   "Base URL for the Smartlogic instance",
		EnvVar: "SMARTLOGIC_BASE_URL",
	})

	smartlogicModel := app.String(cli.StringOpt{
		Name:   "smartlogicModel",
		Desc:   "Smartlogic model to read from",
		EnvVar: "SMARTLOGIC_MODEL",
	})

	smartlogicAPIKey := app.String(cli.StringOpt{
		Name:   "smartlogicAPIKey",
		Desc:   "Smartlogic API key",
		EnvVar: "SMARTLOGIC_API_KEY",
	})

	smartlogicTimeout := app.String(cli.StringOpt{
		Name:   "smartlogicTimeout",
		Desc:   "Number of seconds to wait for smartlogic to respond to our requests",
		EnvVar: "SMARTLOGIC_TIMEOUT",
		Value:  "30s",
	})

	smartlogicHealthcheckConcept := app.String(cli.StringOpt{
		Name:   "smartlogicHealthcheckConcept",
		Desc:   "Concept uuid existing in the Smartlogic model to be used for healthcheck",
		EnvVar: "SMARTLOGIC_HEALTHCHECK_CONCEPT",
	})

	port := app.String(cli.StringOpt{
		Name:   "port",
		Value:  "8080",
		Desc:   "Port to listen on",
		EnvVar: "APP_PORT",
	})

	logLevel := app.String(cli.StringOpt{
		Name:   "logLevel",
		Value:  "info",
		Desc:   "Level of logging to be shown",
		EnvVar: "LOG_LEVEL",
	})

	smartlogicHealthCacheFor := app.String(cli.StringOpt{
		Name:   "healthcheckSuccessCacheTime",
		Value:  "1m",
		Desc:   "How long to cache a successful Smartlogic response for",
		EnvVar: "HEALTHCHECK_SUCCESS_CACHE_TIME",
	})

	conceptUriPrefix := app.String(cli.StringOpt{
		Name:   "conceptUriPrefix",
		Value:  "http://www.ft.com/thing/",
		Desc:   "The concept URI prefix to be added before the UUID part of the Smartlogic request path",
		EnvVar: "CONCEPT_URI_PREFIX",
	})

	log := logger.NewUPPLogger(*appName, *logLevel)
	log.Infof("[Startup] %s is starting", *appSystemCode)

	smartlogicHealthCacheDuration, err := time.ParseDuration(*smartlogicHealthCacheFor)
	if err != nil {
		log.Warnf("Health check success cache duration %s could not be parsed", *smartlogicHealthCacheFor)
		smartlogicHealthCacheDuration = time.Duration(time.Minute)
	}

	smartlogicTimeoutDuration, err := time.ParseDuration(*smartlogicTimeout)
	if err != nil {
		log.WithError(err).Fatalf("Smartlogic timeout duration %s could not be parsed", *smartlogicTimeout)
	}

	if *smartlogicBaseURL == "" {
		log.Fatalf("Failed to start the service, smartlogicBaseURL is required.")
	}
	if *smartlogicModel == "" {
		log.Fatalf("Failed to start the service, smartlogicModel is required.")
	}
	if *smartlogicAPIKey == "" {
		log.Fatalf("Failed to start the service, smartlogicAPIKey is required.")
	}
	if *smartlogicHealthcheckConcept == "" {
		log.Fatalf("Failed to start the service, smartlogicHealthcheckConcept is required.")
	}

	log.Infof("Caching successful health for %s", smartlogicHealthCacheDuration)
	log.Infof("Checking Smartlogic health via getting concept %s of model %s", *smartlogicHealthcheckConcept, *smartlogicModel)

	app.Action = func() {
		log.Infof("System code: %s, App Name: %s, Port: %s", *appSystemCode, *appName, *port)

		router := mux.NewRouter()

		producerConfig := kafka.ProducerConfig{
			Topic:                   *kafkaTopic,
			BrokersConnectionString: *kafkaAddresses,
			Options:                 kafka.DefaultProducerOptions(),
		}
		producer := kafka.NewProducer(producerConfig, log)
		httpClient := getResilientClient(smartlogicTimeoutDuration)
		slClient, err := smartlogic.NewSmartlogicClient(httpClient, *smartlogicBaseURL, *smartlogicModel, *smartlogicAPIKey, *conceptUriPrefix, log)
		if err != nil {
			log.Error("Error generating access token when connecting to Smartlogic.  If this continues to fail, please check the configuration.")
		}

		service := notifier.NewNotifierService(producer, slClient, log)

		handler := notifier.NewNotifierHandler(service, *smartlogicModel, log)
		handler.RegisterEndpoints(router)

		healthServiceConfig := &notifier.HealthServiceConfig{
			AppSystemCode:          *appSystemCode,
			AppName:                *appName,
			Description:            appDescription,
			SmartlogicModel:        *smartlogicModel,
			SmartlogicModelConcept: *smartlogicHealthcheckConcept,
			SuccessCacheTime:       smartlogicHealthCacheDuration,
		}
		healthService, err := notifier.NewHealthService(service, healthServiceConfig, log)
		if err != nil {
			log.Fatalf("Failed to initialize health check service: %v", err)
		}
		healthService.Start()
		monitoringRouter := healthService.RegisterAdminEndpoints(router)

		go func() {
			if err := http.ListenAndServe(":"+*port, monitoringRouter); err != nil {
				log.Fatalf("Unable to start: %v", err)
			}
		}()

		waitForSignal()
	}
	err = app.Run(os.Args)
	if err != nil {
		log.Errorf("App could not start, error=[%s]\n", err)
		return
	}
}

func waitForSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
}

func getResilientClient(timeout time.Duration) *pester.Client {
	c := &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 10,
			MaxIdleConns:        10,
		},
		Timeout: timeout,
	}
	client := pester.NewExtendedClient(c)
	client.Backoff = pester.ExponentialBackoff
	client.MaxRetries = 5
	client.Concurrency = 1

	return client
}
