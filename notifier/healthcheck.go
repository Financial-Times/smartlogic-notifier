package notifier

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/http-handlers-go/httphandlers"
	"github.com/Financial-Times/service-status-go/gtg"
	status "github.com/Financial-Times/service-status-go/httphandlers"
	"github.com/gorilla/mux"
	"github.com/rcrowley/go-metrics"
	log "github.com/sirupsen/logrus"
)

const (
	businessImpact = "Editorial updates of concepts in Smartlogic will not be ingested into UPP"
	panicGuideURL  = "https://runbooks.in.ft.com/smartlogic-notifier"
)

// HealthService is responsible for gtg and health checks.
type HealthService struct {
	sync.RWMutex
	config            *HealthServiceConfig
	notifier          Servicer
	Checks            []fthealth.Check
	checkSuccessCache bool
}

type HealthServiceConfig struct {
	AppSystemCode          string
	AppName                string
	Description            string
	SmartlogicModel        string
	SmartlogicModelConcept string
	SuccessCacheTime       time.Duration
}

func (c *HealthServiceConfig) Validate() error {
	if c.AppSystemCode == "" {
		return errors.New("property AppSystemCode is required")
	}
	if c.AppName == "" {
		return errors.New("property AppName is required")
	}
	if c.Description == "" {
		return errors.New("property Description is required")
	}
	if c.SmartlogicModel == "" {
		return errors.New("property SmartlogicModel is required")
	}
	if c.SmartlogicModelConcept == "" {
		return errors.New("property SmartlogicModelConcept is required")
	}
	if c.SuccessCacheTime.Nanoseconds() <= 0 {
		return errors.New("property SuccessCacheTime is required")
	}
	return nil
}

// NewHealthService initialises the HealthCheck service but doesn't start the updating of the health check result.
func NewHealthService(notifier Servicer, config *HealthServiceConfig) (*HealthService, error) {
	err := config.Validate()
	if err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	service := &HealthService{
		config:   config,
		notifier: notifier,
	}
	service.Checks = []fthealth.Check{
		service.kafkaHealthCheck(),
		service.smartlogicHealthCheck(),
	}
	return service, nil
}

// Start starts separate go routine responsible for updating the cached result of the gtg/health check.
func (hs *HealthService) Start() {
	go func() {
		// perform connectivity check and cache the result
		err := hs.updateSmartlogicSuccessCache()
		if err != nil {
			log.WithError(err).Error("could not perform Smartlogic connectivity check")
		}
		ticker := time.NewTicker(hs.config.SuccessCacheTime)
		defer ticker.Stop()
		for range ticker.C {
			err := hs.updateSmartlogicSuccessCache()
			if err != nil {
				log.WithError(err).Error("could not perform latest Smartlogic connectivity check")
			}
		}
	}()
}

// updateSmartlogicSuccessCache tries to get concept from the Smartlogic model, which uuid is given in the config
// of the health check service, and based on the success of the check updates the HealthService cache.
func (hs *HealthService) updateSmartlogicSuccessCache() error {
	_, err := hs.notifier.GetConcept(hs.config.SmartlogicModelConcept)
	if err != nil {
		log.WithError(err).Errorf("health check concept %s couldn't be retrieved", hs.config.SmartlogicModelConcept)
		hs.setCheckSuccessCache(false)
		return err
	}
	hs.setCheckSuccessCache(true)
	return nil
}

// RegisterAdminEndpoints adds the admin endpoints to the given router
func (hs *HealthService) RegisterAdminEndpoints(router *mux.Router) http.Handler {
	router.HandleFunc("/__health", fthealth.Handler(hs.HealthcheckHandler()))
	router.HandleFunc(status.GTGPath, status.NewGoodToGoHandler(hs.GtgCheck()))
	router.HandleFunc(status.BuildInfoPath, status.BuildInfoHandler)

	var monitoringRouter http.Handler = router
	monitoringRouter = httphandlers.TransactionAwareRequestLoggingHandler(log.StandardLogger(), monitoringRouter)
	monitoringRouter = httphandlers.HTTPMetricsHandler(metrics.DefaultRegistry, monitoringRouter)

	return monitoringRouter
}

// HealthcheckHandler is resposible for __health endpoint.
func (hs *HealthService) HealthcheckHandler() fthealth.TimedHealthCheck {
	return fthealth.TimedHealthCheck{
		HealthCheck: fthealth.HealthCheck{
			SystemCode:  hs.config.AppSystemCode,
			Name:        hs.config.AppName,
			Description: hs.config.Description,
			Checks:      hs.Checks,
		},
		Timeout: 10 * time.Second,
	}
}

func (hs *HealthService) smartlogicHealthCheck() fthealth.Check {
	return fthealth.Check{
		BusinessImpact:   businessImpact,
		Name:             fmt.Sprintf("Check connectivity to Smartlogic model %s", hs.config.SmartlogicModel),
		PanicGuide:       panicGuideURL,
		Severity:         3,
		TechnicalSummary: `Check that Smartlogic is healthy and the API is accessible.  If it is, restart this service.`,
		Checker:          hs.smartlogicConnectivityCheck,
	}
}

func (hs *HealthService) kafkaHealthCheck() fthealth.Check {
	return fthealth.Check{
		BusinessImpact:   businessImpact,
		Name:             "Check connectivity to Kafka",
		PanicGuide:       panicGuideURL,
		Severity:         3,
		TechnicalSummary: `Cannot connect to Kafka. Verify that Kafka is healthy in this cluster.`,
		Checker:          hs.checkKafkaConnectivity,
	}
}

// smartlogicConnectivityCheck always returns the cached result for the Smartlogic connectivity check.
func (hs *HealthService) smartlogicConnectivityCheck() (string, error) {
	if !hs.getCheckSuccessCache() {
		msg := "latest Smartlogic connectivity check is unsuccessful"
		log.Error(msg)
		return msg, errors.New(msg)
	}
	return "", nil
}

func (hs *HealthService) checkKafkaConnectivity() (string, error) {
	err := hs.notifier.CheckKafkaConnectivity()
	if err != nil {
		clientError := fmt.Sprint("Error verifying open connection to Kafka")
		log.WithError(err).Error(clientError)
		return "Error connecting with Kafka", errors.New(clientError)
	} else {
		return "Successfully connected to Kafka", nil
	}
}

// GtgCheck is responsible for __gtg endpoint.
func (hs *HealthService) GtgCheck() gtg.StatusChecker {
	var sc []gtg.StatusChecker
	for _, c := range hs.Checks {
		sc = append(sc, gtgCheck(c.Checker))
	}

	return gtg.FailFastParallelCheck(sc)
}

func (hs *HealthService) getCheckSuccessCache() bool {
	hs.RLock()
	defer hs.RUnlock()
	return hs.checkSuccessCache
}

func (hs *HealthService) setCheckSuccessCache(val bool) {
	hs.Lock()
	defer hs.Unlock()
	hs.checkSuccessCache = val
}

func gtgCheck(handler func() (string, error)) gtg.StatusChecker {
	return func() gtg.Status {
		if _, err := handler(); err != nil {
			return gtg.Status{GoodToGo: false, Message: err.Error()}
		}
		return gtg.Status{GoodToGo: true}
	}
}
