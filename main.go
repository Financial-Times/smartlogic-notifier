package main

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Financial-Times/kafka-client-go/kafka"
	"github.com/Financial-Times/smartlogic-notifier/notifier"
	"github.com/Financial-Times/smartlogic-notifier/smartlogic"
	"github.com/gorilla/mux"
	"github.com/jawher/mow.cli"
	_ "github.com/joho/godotenv/autoload"
	"github.com/sethgrid/pester"
	log "github.com/sirupsen/logrus"
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
		Desc:   "Comma separated list of Kafka broker addresses",
		EnvVar: "KAFKA_ADDRESSES",
	})

	kafkaTopic := app.String(cli.StringOpt{
		Name:   "kafkaTopic",
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
		Desc:   "Smartlogic model to read from",
		EnvVar: "SMARTLOGIC_API_KEY",
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

	lvl, err := log.ParseLevel(*logLevel)
	if err != nil {
		log.Warnf("Log level %s could not be parsed, defaulting to info", *logLevel)
		lvl = log.InfoLevel
	}
	log.SetLevel(lvl)
	log.SetFormatter(&log.JSONFormatter{})
	log.Infof("[Startup] %s is starting", *appSystemCode)

	smartlogicHealthCacheDuration, err := time.ParseDuration(*smartlogicHealthCacheFor)
	if err != nil {
		log.Warnf("Health check success cache duration %s could not be parsed", *smartlogicHealthCacheFor)
		smartlogicHealthCacheDuration = time.Duration(time.Minute)
	}

	log.Infof("Caching successful health for %s", smartlogicHealthCacheDuration)

	app.Action = func() {
		log.Infof("System code: %s, App Name: %s, Port: %s", *appSystemCode, *appName, *port)

		router := mux.NewRouter()

		kf, err := kafka.NewProducer(*kafkaAddresses, *kafkaTopic, kafka.DefaultProducerConfig())
		if err != nil {
			log.WithField("kafkaAddresses", *kafkaAddresses).WithField("kafkaTopic", *kafkaTopic).Fatalf("Error creating the Kafka producer.")
		}
		httpClient := getResilientClient()
		sl, err := smartlogic.NewSmartlogicClient(httpClient, *smartlogicBaseURL, *smartlogicModel, *smartlogicAPIKey)
		if err != nil {
			log.Error("Error generating access token when connecting to Smartlogic.  If this continues to fail, please check the configuration.")
		}

		service := notifier.NewNotifierService(kf, sl)

		handler := notifier.NewNotifierHandler(service)
		handler.RegisterEndpoints(router)
		monitoringRouter := handler.RegisterAdminEndpoints(
			router,
			*appSystemCode,
			*appName,
			appDescription,
			smartlogicHealthCacheDuration,
		)

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
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
}

func getResilientClient() *pester.Client {
	c := &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 10,
			MaxIdleConns:        10,
		},
		Timeout: 5 * time.Second,
	}
	client := pester.NewExtendedClient(c)
	client.Backoff = pester.ExponentialBackoff
	client.MaxRetries = 5
	client.Concurrency = 1

	return client
}
