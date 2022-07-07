package notifier

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

func TestNewHealthService(t *testing.T) {
	tests := []struct {
		name          string
		config        HealthServiceConfig
		expectedError bool
	}{
		{
			name: "success",
			config: HealthServiceConfig{
				AppSystemCode:          "test-smartlogic-notifier",
				AppName:                "test-smartlogic-notifier",
				Description:            "test description",
				SmartlogicModel:        "TestSmartlogicModel",
				SmartlogicModelConcept: "b1a492d9-dcfe-43f8-8072-17b4618a78fd",
				SuccessCacheTime:       1 * time.Minute,
			},
			expectedError: false,
		},
		{
			name: "missing app system code",
			config: HealthServiceConfig{
				AppSystemCode:          "",
				AppName:                "test-smartlogic-notifier",
				Description:            "test description",
				SmartlogicModel:        "TestSmartlogicModel",
				SmartlogicModelConcept: "b1a492d9-dcfe-43f8-8072-17b4618a78fd",
				SuccessCacheTime:       1 * time.Minute,
			},
			expectedError: true,
		},
		{
			name: "missing app name",
			config: HealthServiceConfig{
				AppSystemCode:          "test-smartlogic-notifier",
				AppName:                "",
				Description:            "test description",
				SmartlogicModel:        "TestSmartlogicModel",
				SmartlogicModelConcept: "b1a492d9-dcfe-43f8-8072-17b4618a78fd",
				SuccessCacheTime:       1 * time.Minute,
			},
			expectedError: true,
		},
		{
			name: "missing description",
			config: HealthServiceConfig{
				AppSystemCode:          "test-smartlogic-notifier",
				AppName:                "test-smartlogic-notifier",
				Description:            "",
				SmartlogicModel:        "TestSmartlogicModel",
				SmartlogicModelConcept: "b1a492d9-dcfe-43f8-8072-17b4618a78fd",
				SuccessCacheTime:       1 * time.Minute,
			},
			expectedError: true,
		},
		{
			name: "missing Smartlogic model",
			config: HealthServiceConfig{
				AppSystemCode:          "test-smartlogic-notifier",
				AppName:                "test-smartlogic-notifier",
				Description:            "test description",
				SmartlogicModel:        "",
				SmartlogicModelConcept: "b1a492d9-dcfe-43f8-8072-17b4618a78fd",
				SuccessCacheTime:       1 * time.Minute,
			},
			expectedError: true,
		},
		{
			name: "missing healthcheck concept",
			config: HealthServiceConfig{
				AppSystemCode:          "test-smartlogic-notifier",
				AppName:                "test-smartlogic-notifier",
				Description:            "test description",
				SmartlogicModel:        "TestSmartlogicModel",
				SmartlogicModelConcept: "",
				SuccessCacheTime:       1 * time.Minute,
			},
			expectedError: true,
		},
		{
			name: "missing cache time period",
			config: HealthServiceConfig{
				AppSystemCode:          "test-smartlogic-notifier",
				AppName:                "test-smartlogic-notifier",
				Description:            "test description",
				SmartlogicModel:        "TestSmartlogicModel",
				SmartlogicModelConcept: "b1a492d9-dcfe-43f8-8072-17b4618a78fd",
				SuccessCacheTime:       0,
			},
			expectedError: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewHealthService(&mockService{}, &test.config, logger.NewUnstructuredLogger())
			if err != nil && !test.expectedError {
				t.Errorf("unexpected error initializing HealthService: %v", err)
			}
			if err == nil && test.expectedError {
				t.Error("expected error initializing HealthService")
			}
		})
	}
}

func TestHealthServiceChecks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		url            string
		mockService    *mockService
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "gtg endpoint success",
			url:  "__gtg",
			mockService: &mockService{
				getConcept: func(s string) ([]byte, error) {
					return []byte(""), nil
				},
				checkKafkaConnectivity: func() error {
					return nil
				},
			},
			expectedStatus: 200,
			expectedBody:   "OK",
		},
		{
			name: "health endpoint success",
			url:  "__health",
			mockService: &mockService{
				getConcept: func(s string) ([]byte, error) {
					return []byte(""), nil
				},
				checkKafkaConnectivity: func() error {
					return nil
				},
			},
			expectedStatus: 200,
			expectedBody:   `"ok":true}`,
		},
		{
			name: "gtg endpoint Smartlogic failure",
			url:  "__gtg",
			mockService: &mockService{
				getConcept: func(s string) ([]byte, error) {
					return nil, errors.New("couldn't retrieve FT organisation from Smartlogic")
				},
				checkKafkaConnectivity: func() error {
					return nil
				},
			},
			expectedStatus: 503,
			expectedBody:   "latest Smartlogic connectivity check is unsuccessful",
		},
		{
			name: "gtg endpoint Kafka failure",
			url:  "__gtg",
			mockService: &mockService{
				getConcept: func(s string) ([]byte, error) {
					return []byte(""), nil
				},
			},
			expectedStatus: 503,
			expectedBody:   "Error verifying open connection to Kafka",
		},
		{
			name: "health endpoint Smartlogic failure",
			url:  "__health",
			mockService: &mockService{
				checkKafkaConnectivity: func() error {
					return nil
				},
			},
			expectedStatus: 200, // the __health endpoint always returns 200
			expectedBody:   `"ok":false,"severity":3}`,
		},
		{
			name: "health endpoint Kafka failure",
			url:  "__health",
			mockService: &mockService{
				getConcept: func(s string) ([]byte, error) {
					return []byte(""), nil
				},
			},
			expectedStatus: 200, // the __health endpoint always returns 200
			expectedBody:   `"ok":false,"severity":3}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m := mux.NewRouter()
			healthcheckCacheInterval := 10 * time.Millisecond
			healthConfig := &HealthServiceConfig{
				AppSystemCode:          "system-code",
				AppName:                "app-name",
				Description:            "description",
				SmartlogicModel:        "testModel",
				SmartlogicModelConcept: "testConcept",
				SuccessCacheTime:       healthcheckCacheInterval,
			}
			healthService, err := NewHealthService(test.mockService, healthConfig, logger.NewUnstructuredLogger())
			if err != nil {
				t.Fatal(err)
			}
			healthService.Start()
			_ = healthService.RegisterAdminEndpoints(m)

			// give time the cache of the Healthcheck service to be updated to be updated (getConcept to be called)
			time.Sleep(healthcheckCacheInterval)

			assertRequest(t, m, test.url, test.expectedBody, test.expectedStatus)
		})
	}
}

func TestHealthServiceCache(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                  string
		url                   string
		expectedFailureStatus int
		expectedSuccessBody   string
		expectedFailureBody   string
	}{
		{
			name:                  "Test the cache for __health endpoint",
			url:                   "__health",
			expectedFailureStatus: 200,
			expectedSuccessBody:   `"ok":true`,
			expectedFailureBody:   `"ok":false`,
		},
		{
			name:                  "Test the cache for __gtg endpoint",
			url:                   "__gtg",
			expectedFailureStatus: 503,
			expectedSuccessBody:   "OK",
			expectedFailureBody:   "latest Smartlogic connectivity check is unsuccessful",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ok := true
			okMutex := sync.RWMutex{}
			getConcept := func(s string) ([]byte, error) {
				okMutex.Lock()
				defer okMutex.Unlock()
				if ok {
					return []byte(""), nil
				}
				return nil, errors.New("FT concept couldn't be retrieved")
			}

			mockSvc := &mockService{
				getConcept: getConcept,
				checkKafkaConnectivity: func() error {
					return nil
				},
			}
			m := mux.NewRouter()

			healthcheckCacheInterval := 20 * time.Millisecond
			healthConfig := &HealthServiceConfig{
				AppSystemCode:          "system-code",
				AppName:                "app-name",
				Description:            "description",
				SmartlogicModel:        "testModel",
				SmartlogicModelConcept: "testConcept",
				SuccessCacheTime:       healthcheckCacheInterval,
			}
			healthService, err := NewHealthService(mockSvc, healthConfig, logger.NewUnstructuredLogger())
			if err != nil {
				t.Fatal(err)
			}
			healthService.Start()
			_ = healthService.RegisterAdminEndpoints(m)
			// give time the cache of the Healthcheck service to be updated to be updated (getConcept to be called)
			time.Sleep(healthcheckCacheInterval / 2)

			// check we return ok
			assertRequest(t, m, test.url, test.expectedSuccessBody, 200)

			// tell GetConcept to return error, mocking we couldn't get the FT concept from Smartlogic
			okMutex.Lock()
			ok = false
			okMutex.Unlock()

			// but expect to return cached ok
			assertRequest(t, m, test.url, test.expectedSuccessBody, 200)

			// wait for cache to clear
			time.Sleep(3 * healthcheckCacheInterval / 2)

			// and expect gtg to return err
			assertRequest(t, m, test.url, test.expectedFailureBody, test.expectedFailureStatus)

			// tell GetConcept to return okay
			okMutex.Lock()
			ok = true
			okMutex.Unlock()
			// wait for cache to clear
			time.Sleep(3 * healthcheckCacheInterval / 2)

			// and expect gtg to return ok
			assertRequest(t, m, test.url, test.expectedSuccessBody, 200)
		})
	}
}

func assertRequest(t *testing.T, m http.Handler, url string, expectedBody string, expectedStatus int) {
	req, err := http.NewRequest("GET", "/"+url, bytes.NewBufferString(""))
	if err != nil {
		t.Fatalf("failed creating new test requst to %s", url)
	}
	rr := httptest.NewRecorder()
	m.ServeHTTP(rr, req)
	b, err := ioutil.ReadAll(rr.Body)
	assert.NoError(t, err)
	body := string(b)
	assert.Equal(t, expectedStatus, rr.Code, url)
	assert.Contains(t, body, expectedBody, url)
}
