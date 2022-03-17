package notifier

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewNotifierService(t *testing.T) {
	kc := &mockKafkaClient{}
	sl := &mockSmartlogicClient{}

	service := NewNotifierService(kc, sl)
	assert.IsType(t, &Service{}, service)
}

func TestService_GetConcept(t *testing.T) {
	kc := &mockKafkaClient{}
	sl := &mockSmartlogicClient{
		concepts: map[string]string{
			"uuid1": "concept1",
			"uuid2": "concept2",
		},
		getChangedConceptListFunc: func(changeDate time.Time) ([]string, error) {
			return []string{}, nil
		},
	}

	service := NewNotifierService(kc, sl)

	concept, err := service.GetConcept("uuid2")
	assert.NoError(t, err)
	assert.EqualValues(t, "concept2", string(concept))
}

func TestService_Notify(t *testing.T) {
	kc := &mockKafkaClient{}
	sl := &mockSmartlogicClient{
		concepts: map[string]string{
			"uuid1": "concept1",
			"uuid2": "concept2",
		},
		getChangedConceptListFunc: func(changeDate time.Time) ([]string, error) {
			return []string{"uuid2"}, nil
		},
	}

	service := NewNotifierService(kc, sl)

	err := service.Notify(context.TODO(), time.Now(), "transactionID")

	assert.NoError(t, err)
	assert.Equal(t, 1, kc.sentCount)
}

func TestService_RetryNotify(t *testing.T) {
	var isGetFuncCalled bool
	kc := &mockKafkaClient{}
	sl := &mockSmartlogicClient{
		concepts: map[string]string{
			"uuid1": "concept1",
			"uuid2": "concept2",
		},
		getChangedConceptListFunc: func(changeDate time.Time) ([]string, error) {
			if isGetFuncCalled {
				return []string{"uuid2"}, nil
			}

			isGetFuncCalled = true
			return []string{}, nil
		},
	}

	service := NewNotifierService(kc, sl)

	err := service.Notify(context.TODO(), time.Now(), "transactionID")

	assert.NoError(t, err)
	assert.Equal(t, 1, kc.sentCount)
}

func TestService_ForceNotify(t *testing.T) {
	kc := &mockKafkaClient{}
	sl := &mockSmartlogicClient{
		concepts: map[string]string{
			"uuid1": "concept1",
			"uuid2": "concept2",
		},
		getChangedConceptListFunc: func(changeDate time.Time) ([]string, error) {
			return []string{}, nil
		},
	}

	service := NewNotifierService(kc, sl)

	err := service.ForceNotify(context.TODO(), []string{"uuid1"}, "transactionID")

	assert.NoError(t, err)
	assert.Equal(t, 1, kc.sentCount)
}
