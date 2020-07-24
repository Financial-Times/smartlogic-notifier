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
	GetChangedConceptList(lastChange time.Time) ([]string, error)
	Notify(lastChange time.Time, transactionID string) error
	ForceNotify(UUIDs []string, transactionID string) error
	CheckKafkaConnectivity() error
}

type Service struct {
	kafka    kafka.Producer
	slClient smartlogic.Clienter
}

func NewNotifierService(kafka kafka.Producer, slClient smartlogic.Clienter) *Service {
	return &Service{
		kafka:    kafka,
		slClient: slClient,
	}
}

func (s *Service) GetConcept(uuid string) ([]byte, error) {
	return s.slClient.GetConcept(uuid)
}

func (s *Service) GetChangedConceptList(lastChange time.Time) (uuids []string, err error) {
	return s.slClient.GetChangedConceptList(lastChange)
}

func (s *Service) Notify(lastChange time.Time, transactionID string) error {
	changedConcepts, err := s.slClient.GetChangedConceptList(lastChange)
	if err != nil {
		return fmt.Errorf("failed to fetch the list of changed concepts: %w", err)
	}
	if len(changedConcepts) == 0 {
		return fmt.Errorf("no changed concepts since %v were returned for transaction id %s", lastChange, transactionID)
	}

	return s.ForceNotify(changedConcepts, transactionID)
}

func (s *Service) ForceNotify(UUIDs []string, transactionID string) error {
	errorMap := map[string]error{}

	for _, conceptUUID := range UUIDs {
		concept, err := s.slClient.GetConcept(conceptUUID)
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
			"concept_uuid":           conceptUUID,
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
