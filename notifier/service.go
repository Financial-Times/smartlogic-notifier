package notifier

import (
	"errors"
	"fmt"
	"time"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/kafka-client-go/v4"
	"github.com/Financial-Times/smartlogic-notifier/smartlogic"
	transactionidutils "github.com/Financial-Times/transactionid-utils-go"
)

type Servicer interface {
	GetConcept(uuid string) ([]byte, error)
	GetChangedConceptList(lastChange time.Time) ([]string, error)
	Notify(lastChange time.Time, transactionID string) error
	ForceNotify(UUIDs []string, transactionID string) error
	CheckKafkaConnectivity() error
}

type Service struct {
	producer messageProducer
	slClient smartlogic.Clienter
	log      *logger.UPPLogger
}

type messageProducer interface {
	SendMessage(message kafka.FTMessage) error
	ConnectivityCheck() error
}

func NewNotifierService(producer messageProducer, slClient smartlogic.Clienter, log *logger.UPPLogger) *Service {
	return &Service{
		producer: producer,
		slClient: slClient,
		log:      log,
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
		// After some time interval retry getting the changed concept list,
		// because Smartlogic sometimes notify us before the data is available to be retrieved.
		time.Sleep(time.Second * 5)
		changedConcepts, err = s.slClient.GetChangedConceptList(lastChange)
		if err != nil {
			return fmt.Errorf("failed while retrying to fetch the list of changed concepts: %w", err)
		}
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
		s.log.
			WithTransactionID(transactionID).
			WithField("concept_transaction_id", newTransactionID).
			WithField("concept_uuid", conceptUUID).
			Info("Sending message to Kafka")
		err = s.producer.SendMessage(message)
		if err != nil {
			errorMap[conceptUUID] = err
		}
	}

	if len(errorMap) > 0 {
		errorMsg := fmt.Sprintf("There was an error with %d concept ingestions", len(errorMap))
		s.log.WithField("errorMap", errorMap).Error(errorMsg)
		return errors.New(errorMsg)
	}
	if len(UUIDs) > 0 {
		s.log.WithField("uuids", UUIDs).Info("Completed notification of concepts")
	}
	return nil
}

func (s *Service) CheckKafkaConnectivity() error {
	return s.producer.ConnectivityCheck()
}
