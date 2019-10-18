package notifier

import (
	"errors"
	"fmt"
	"time"

	"github.com/Financial-Times/kafka-client-go/kafka"
	"github.com/Financial-Times/smartlogic-notifier/smartlogic"
	transactionidutils "github.com/Financial-Times/transactionid-utils-go"
	log "github.com/sirupsen/logrus"
)

type Servicer interface {
	GetConcept(uuid string) ([]byte, error)
	Notify(lastChange time.Time, transactionID string) error
	ForceNotify(UUIDs []string, transactionID string) error
	CheckKafkaConnectivity() error
}

type Service struct {
	kafka      kafka.Producer
	smartlogic smartlogic.Clienter
}

func NewNotifierService(kafka kafka.Producer, smartlogic smartlogic.Clienter) Servicer {
	return &Service{
		kafka:      kafka,
		smartlogic: smartlogic,
	}
}

func (s *Service) GetConcept(uuid string) ([]byte, error) {
	return s.smartlogic.GetConcept(uuid)
}

func (s *Service) Notify(lastChange time.Time, transactionID string) error {
	changedConcepts, err := s.smartlogic.GetChangedConceptList(lastChange)
	if err != nil {
		log.WithError(err).Error("There was an error retrieving the list of changed concepts")
		return err
	}

	return s.ForceNotify(changedConcepts, transactionID)
}

func (s *Service) ForceNotify(UUIDs []string, transactionID string) error {
	errorMap := map[string]error{}

	for _, conceptUUID := range UUIDs {
		concept, err := s.smartlogic.GetConcept(conceptUUID)
		if err != nil {
			errorMap[conceptUUID] = err
			continue
		}

		newTransactionID := transactionidutils.NewTransactionID()

		message := kafka.NewFTMessage(map[string]string{
			transactionidutils.TransactionIDHeader: newTransactionID,
		}, string(concept))

		log.WithFields(log.Fields{
			"request_transaction_id": transactionID,
			"concept_transaction_id": newTransactionID,
		}).Info("Sending message to Kafka")
		err = s.kafka.SendMessage(message)
		if err != nil {
			errorMap[conceptUUID] = err
		}
	}

	if len(errorMap) > 0 {
		errorMsg := fmt.Sprintf("There was an error with %d concept ingestions", len(errorMap))
		log.WithField("errorMap", errorMap).Error(errorMsg)
		return errors.New(errorMsg)
	}
	if len(UUIDs) > 0 {
		log.WithField("uuids", UUIDs).Info("Completed notification of concepts")
	}
	return nil
}

func (s *Service) CheckKafkaConnectivity() error {
	return s.kafka.ConnectivityCheck()
}
