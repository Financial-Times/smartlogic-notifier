package notifier

import (
	"bytes"
	"errors"
	"io/ioutil"
	http "net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestHandlers(t *testing.T) {
	log.SetOutput(ioutil.Discard)

	testCases := []struct {
		name        string
		method      string
		url         string
		requestBody string
		resultCode  int
		resultBody  string
		mockService *MockService
	}{
		{
			name:       "Notify - Success",
			method:     "GET",
			url:        "/notify?affectedGraphId=1&modifiedGraphId=2&lastChangeDate=2017-05-31T13:00:00.000Z",
			resultCode: 200,
			resultBody: "{\"message\": \"Messages successfully sent to Kafka\"}",
			mockService: &MockService{
				notify: func(i time.Time, s string) error {
					return nil
				},
			},
		},
		{
			name:        "Notify - Missing query parameters",
			method:      "GET",
			url:         "/notify?modifiedGraphId=2&lastChangeDate=2017-05-31T13:00:00.000Z",
			resultCode:  400,
			resultBody:  "{\"message\": \"Query parameters were not set: affectedGraphId\"}",
			mockService: &MockService{},
		},
		{
			name:        "Notify - Missing all query parameters",
			method:      "GET",
			url:         "/notify",
			resultCode:  400,
			resultBody:  "{\"message\": \"Query parameters were not set: modifiedGraphId, affectedGraphId, lastChangeDate\"}",
			mockService: &MockService{},
		},
		{
			name:        "Notify - Bad query parameters ",
			method:      "GET",
			url:         "/notify?affectedGraphId=1&modifiedGraphId=2&lastChangeDate=notadate",
			resultCode:  400,
			resultBody:  "{\"message\": \"Date is not in the format 2006-01-02T15:04:05.000Z\"}",
			mockService: &MockService{},
		},
		{
			name:       "Notify - Error",
			method:     "GET",
			url:        "/notify?affectedGraphId=1&modifiedGraphId=2&lastChangeDate=2017-05-31T13:00:00.000Z",
			resultCode: 500,
			resultBody: "{\"message\": \"There was an error completing the notify\", \"error\": \"anerror\"}",
			mockService: &MockService{
				notify: func(i time.Time, s string) error {
					return errors.New("anerror")
				},
			},
		},
		{
			name:        "Force Notify - Success",
			method:      "POST",
			url:         "/force-notify",
			requestBody: `{"uuids": ["1","2","3"]}`,
			resultCode:  200,
			resultBody:  "Concept notification completed",
			mockService: &MockService{
				forceNotify: func(strings []string, s string) error {
					return nil
				},
			},
		},
		{
			name:        "Force Notify - Bad Payload",
			method:      "POST",
			url:         "/force-notify",
			requestBody: `{"uuids": "1","2","3"]}`,
			resultCode:  400,
			resultBody:  "{\"message\": \"There was an error decoding the payload\", \"error\": \"invalid character ',' after object key\"}",
			mockService: &MockService{},
		},
		{
			name:        "Force Notify - Failure",
			method:      "POST",
			url:         "/force-notify",
			requestBody: `{"uuids": ["1","2","3"]}`,
			resultCode:  500,
			resultBody:  "{\"message\": \"There was an error completing the force notify\"}",
			mockService: &MockService{
				forceNotify: func(strings []string, s string) error {
					return errors.New("error in force notify")
				},
			},
		},
		{
			name:       "Get Concept - Success",
			method:     "GET",
			url:        "/concept/1",
			resultCode: 200,
			resultBody: "1",
			mockService: &MockService{
				getConcept: func(s string) ([]byte, error) {
					return []byte("1"), nil
				},
			},
		},
		{
			name:       "Get Concept - Error",
			method:     "GET",
			url:        "/concept/11",
			resultCode: 500,
			resultBody: "{\"message\": \"There was an error retrieving the concept\", \"error\": \"can't find concept\"}",
			mockService: &MockService{
				getConcept: func(s string) ([]byte, error) {
					return nil, errors.New("can't find concept")
				},
			},
		},
		{
			name:        "__health",
			method:      "GET",
			url:         "/__health",
			resultCode:  200,
			resultBody:  "IGNORE",
			mockService: &MockService{},
		},
		{
			name:        "__build-info",
			method:      "GET",
			url:         "/__build-info",
			resultCode:  200,
			resultBody:  "IGNORE",
			mockService: &MockService{},
		},
		{
			name:        "__gtg",
			method:      "GET",
			url:         "/__gtg",
			resultCode:  503,
			resultBody:  "IGNORE",
			mockService: &MockService{},
		},
	}

	for _, d := range testCases {
		t.Run(d.name, func(t *testing.T) {
			handler := NewNotifierHandler(d.mockService)
			m := mux.NewRouter()
			handler.RegisterEndpoints(m)

			healthService := NewHealthService(d.mockService, "system-code", "app-name", "description", "testModel", time.Minute)
			healthService.Start()
			_ = healthService.RegisterAdminEndpoints(m)

			req, _ := http.NewRequest(d.method, d.url, bytes.NewBufferString(d.requestBody))
			rr := httptest.NewRecorder()
			m.ServeHTTP(rr, req)

			b, err := ioutil.ReadAll(rr.Body)
			assert.NoError(t, err)
			body := string(b)
			assert.Equal(t, d.resultCode, rr.Code, d.name)
			if d.resultBody != "IGNORE" {
				assert.Equal(t, d.resultBody, body, d.name)
			}

		})
	}

}

func TestHealthServiceChecks(t *testing.T) {
	log.SetOutput(ioutil.Discard)

	tests := []struct {
		name           string
		url            string
		mockService    *MockService
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "gtg endpoint success",
			url:  "__gtg",
			mockService: &MockService{
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
			mockService: &MockService{
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
			mockService: &MockService{
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
			mockService: &MockService{
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
			mockService: &MockService{
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
			mockService: &MockService{
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
			healthService := NewHealthService(test.mockService, "system-code", "app-name", "description", "testModel", healthcheckCacheInterval)
			healthService.Start()
			_ = healthService.RegisterAdminEndpoints(m)

			// give time the cache of the Healthcheck service to be updated to be updated (getConcept to be called)
			time.Sleep(healthcheckCacheInterval)

			assertRequest(t, m, test.url, test.expectedBody, test.expectedStatus)
		})
	}
}

func TestHealthServiceCache(t *testing.T) {
	log.SetOutput(ioutil.Discard)

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

			mockSvc := &MockService{
				getConcept: getConcept,
				checkKafkaConnectivity: func() error {
					return nil
				},
			}
			m := mux.NewRouter()

			healthcheckCacheInterval := 20 * time.Millisecond
			healthService := NewHealthService(mockSvc, "system-code", "app-name", "description", "testModel", healthcheckCacheInterval)
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
