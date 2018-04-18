package notifier

import (
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
	sl := NewMockSmartlogicClient(map[string]string{
		"uuid1": "concept1",
		"uuid2": "concept2",
	}, []string{})
	service := NewNotifierService(kc, sl)

	concept, err := service.GetConcept("uuid2")
	assert.NoError(t, err)
	assert.EqualValues(t, "concept2", string(concept))
}

func TestService_Notify(t *testing.T) {
	kc := &mockKafkaClient{}
	sl := NewMockSmartlogicClient(map[string]string{
		"uuid1": "concept1",
		"uuid2": "concept2",
	}, []string{"uuid2"})
	service := NewNotifierService(kc, sl)

	err := service.Notify(time.Now(), "transactionID")

	assert.NoError(t, err)
	assert.Equal(t, 1, kc.sentCount)
}

func TestService_ForceNotify(t *testing.T) {
	kc := &mockKafkaClient{}
	sl := NewMockSmartlogicClient(map[string]string{
		"uuid1": "concept1",
		"uuid2": "concept2",
	}, []string{})
	service := NewNotifierService(kc, sl)

	err := service.ForceNotify([]string{"uuid1"}, "transactionID")

	assert.NoError(t, err)
	assert.Equal(t, 1, kc.sentCount)
}
