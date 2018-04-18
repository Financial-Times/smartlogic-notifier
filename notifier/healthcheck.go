package notifier

import (
	"fmt"
	"sync"
	"time"

	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/service-status-go/gtg"
	log "github.com/sirupsen/logrus"
)

type HealthService struct {
	sync.RWMutex
	config                        *config
	notifier                      Servicer
	Checks                        []fthealth.Check
	lastSuccessfulSmartlogicCheck time.Time
}

type config struct {
	appSystemCode             string
	appName                   string
	description               string
	cacheSmartlogicSuccessFor time.Duration
}

func NewHealthService(notifier Servicer, appSystemCode string, appName string, description string, cacheSmartlogicSuccessFor time.Duration) *HealthService {
	service := &HealthService{
		config: &config{
			appSystemCode:             appSystemCode,
			appName:                   appName,
			description:               description,
			cacheSmartlogicSuccessFor: cacheSmartlogicSuccessFor,
		},
		notifier: notifier,
	}
	service.Checks = []fthealth.Check{
		service.smartlogicHealthCheck(),
	}
	return service
}

func (svc *HealthService) HealthcheckHandler() fthealth.TimedHealthCheck {
	return fthealth.TimedHealthCheck{
		HealthCheck: fthealth.HealthCheck{
			SystemCode:  svc.config.appSystemCode,
			Name:        svc.config.appName,
			Description: svc.config.description,
			Checks:      svc.Checks,
		},
		Timeout: 10 * time.Second,
	}
}

func (svc *HealthService) smartlogicHealthCheck() fthealth.Check {
	return fthealth.Check{
		BusinessImpact:   "Editorial updates of concepts will not be written into UPP",
		Name:             "Check connectivity to Smartlogic",
		PanicGuide:       fmt.Sprintf("https://dewey.ft.com/%s.html", svc.config.appSystemCode),
		Severity:         3,
		TechnicalSummary: `Check that Smartlogic is healthy and the API is accessible.  If it is, restart this service.`,
		Checker:          svc.smartlogicCheck,
	}
}

func (svc *HealthService) smartlogicCheck() (string, error) {
	_, err := svc.notifier.GetConcept("healthcheck-concept")
	if err != nil {
		return "Concept couldn't be retrieved.", err
	}
	return "", nil
}

func (svc *HealthService) GtgCheck() gtg.StatusChecker {
	return gtg.FailFastParallelCheck([]gtg.StatusChecker{
		func() gtg.Status {
			svc.Lock()
			defer svc.Unlock()

			cacheDuration := svc.config.cacheSmartlogicSuccessFor
			nextCheck := svc.lastSuccessfulSmartlogicCheck.Add(cacheDuration)
			if nextCheck.After(time.Now()) {
				log.Debug("Skipping smart logic health check")
				return gtg.Status{GoodToGo: true}
			}
			log.Debug("Performing smart logic health check")
			if _, err := svc.smartlogicCheck(); err != nil {
				return gtg.Status{GoodToGo: false, Message: err.Error()}
			}
			svc.lastSuccessfulSmartlogicCheck = time.Now()
			return gtg.Status{GoodToGo: true}
		},
	})
}
