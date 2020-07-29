package notifier

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Financial-Times/smartlogic-notifier/smartlogic"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

const smartlogicModel = "FTStagingModel"

func TestHandlers(t *testing.T) {
	t.Parallel()
	log.SetOutput(ioutil.Discard)

	today := time.Now().Format(TimeFormat)
	past := time.Date(1900, 1, 1, 0, 0, 0, 0, time.Local).Format(TimeFormat)
	testCases := []struct {
		name        string
		method      string
		url         string
		requestBody string
		resultCode  int
		resultBody  string
		mockService *mockService
	}{
		{
			name:       "Notify - Success",
			method:     "GET",
			url:        fmt.Sprintf("/notify?affectedGraphId=%s&modifiedGraphId=%s&lastChangeDate=%s", smartlogicModel, smartlogicModel, today),
			resultBody: "{\"message\": \"Concepts successfully ingested\"}",
			resultCode: 200,
			mockService: &mockService{
				notify: func(i time.Time, s string) error {
					return nil
				},
			},
		},
		{
			name:        "Notify - Missing query parameters",
			method:      "GET",
			url:         fmt.Sprintf("/notify?modifiedGraphId=%s&lastChangeDate=2017-05-31T13:00:00.000Z", smartlogicModel),
			resultCode:  400,
			resultBody:  "{\"message\": \"Query parameters are missing or incorrect: affectedGraphId\"}",
			mockService: &mockService{},
		},
		{
			name:        "Notify - Missing all query parameters",
			method:      "GET",
			url:         "/notify",
			resultCode:  400,
			resultBody:  "{\"message\": \"Query parameters are missing or incorrect: modifiedGraphId, affectedGraphId, lastChangeDate\"}",
			mockService: &mockService{},
		},
		{
			name:        "Notify - Incorrect modifiedGraphId and affectedGraphId",
			method:      "GET",
			url:         fmt.Sprintf("/notify?affectedGraphId=1&modifiedGraphId=2&lastChangeDate=%s", today),
			resultCode:  400,
			resultBody:  "{\"message\": \"Query parameters are missing or incorrect: modifiedGraphId, affectedGraphId\"}",
			mockService: &mockService{},
		},
		{
			name:        "Notify - Bad query parameters ",
			method:      "GET",
			url:         fmt.Sprintf("/notify?affectedGraphId=%s&modifiedGraphId=%s&lastChangeDate=%s", smartlogicModel, smartlogicModel, "nodate"),
			resultCode:  400,
			resultBody:  "{\"message\": \"Date is not in the format 2006-01-02T15:04:05Z\"}",
			mockService: &mockService{},
		},
		{
			name:        "Notify - Bad lastChangeDate parameter",
			method:      "GET",
			url:         fmt.Sprintf("/notify?affectedGraphId=%s&modifiedGraphId=%s&lastChangeDate=%s", smartlogicModel, smartlogicModel, past),
			resultCode:  400,
			resultBody:  fmt.Sprintf("{\"message\": \"Last change date should be time point in the last %.0f hours\"}", LastChangeLimit.Hours()),
			mockService: &mockService{},
		},
		{
			name:       "Notify - Error",
			method:     "GET",
			url:        fmt.Sprintf("/notify?affectedGraphId=%s&modifiedGraphId=%s&lastChangeDate=%s", smartlogicModel, smartlogicModel, today),
			resultBody: "{\"message\": \"Concepts successfully ingested\"}",
			resultCode: 200,
			mockService: &mockService{
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
			mockService: &mockService{
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
			mockService: &mockService{},
		},
		{
			name:        "Force Notify - Failure",
			method:      "POST",
			url:         "/force-notify",
			requestBody: `{"uuids": ["1","2","3"]}`,
			resultCode:  500,
			resultBody:  "{\"message\": \"There was an error completing the force notify\"}",
			mockService: &mockService{
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
			mockService: &mockService{
				getConcept: func(s string) ([]byte, error) {
					return []byte("1"), nil
				},
			},
		},
		{
			name:       "Get Concept - Error",
			method:     "GET",
			url:        "/concept/11",
			resultCode: 404,
			resultBody: "{\"message\": \"There was an error retrieving the concept\", \"error\": \"concept does not exist\"}",
			mockService: &mockService{
				getConcept: func(s string) ([]byte, error) {
					return nil, smartlogic.ErrorConceptDoesNotExist
				},
			},
		},
		{
			name:       "Get Concept - Error",
			method:     "GET",
			url:        "/concept/11",
			resultCode: 500,
			resultBody: "{\"message\": \"There was an error retrieving the concept\", \"error\": \"failed to get concept\"}",
			mockService: &mockService{
				getConcept: func(s string) ([]byte, error) {
					return nil, errors.New("failed to get concept")
				},
			},
		},
		{
			name:       "Get Concepts - Success",
			method:     "GET",
			url:        fmt.Sprintf("/concepts?lastChangeDate=%s", today),
			resultCode: 200,
			resultBody: `["1","2","3"]`,
			mockService: &mockService{
				getChangedConceptList: func(t time.Time) ([]string, error) {
					return []string{"1", "2", "3"}, nil
				},
			},
		},
		{
			name:        "Get Concepts - Invalid Time",
			method:      "GET",
			url:         fmt.Sprintf("/concepts?lastChangeDate=%s", past),
			resultCode:  400,
			resultBody:  fmt.Sprintf("{\"message\": \"Last change date should be time point in the last %.0f hours\"}", LastChangeLimit.Hours()),
			mockService: &mockService{},
		},
		{
			name:        "Get Concepts - Bad Time format",
			method:      "GET",
			url:         "/concepts?lastChangeDate=nodata",
			resultCode:  400,
			resultBody:  "{\"message\": \"Date is not in the format 2006-01-02T15:04:05Z\"}",
			mockService: &mockService{},
		},
		{
			name:        "Get Concepts - No Time",
			method:      "GET",
			url:         "/concepts",
			resultCode:  400,
			resultBody:  "{\"message\": \"Query parameter lastChangeDate was not set.\"}",
			mockService: &mockService{},
		},
		{
			name:       "Get Concepts - Smartlogic error",
			method:     "GET",
			url:        fmt.Sprintf("/concepts?lastChangeDate=%s", today),
			resultCode: 500,
			resultBody: "{\"message\": \"There was an error getting the changes\", \"error\": \"smartlogic error\"}",
			mockService: &mockService{
				getChangedConceptList: func(t time.Time) ([]string, error) {
					return nil, errors.New("smartlogic error")
				},
			},
		},
		{
			name:        "__health",
			method:      "GET",
			url:         "/__health",
			resultCode:  200,
			resultBody:  "IGNORE",
			mockService: &mockService{},
		},
		{
			name:        "__build-info",
			method:      "GET",
			url:         "/__build-info",
			resultCode:  200,
			resultBody:  "IGNORE",
			mockService: &mockService{},
		},
		{
			name:        "__gtg",
			method:      "GET",
			url:         "/__gtg",
			resultCode:  503,
			resultBody:  "IGNORE",
			mockService: &mockService{},
		},
	}

	for _, d := range testCases {
		t.Run(d.name, func(t *testing.T) {
			handler := NewNotifierHandler(d.mockService, smartlogicModel)
			m := mux.NewRouter()
			handler.RegisterEndpoints(m)

			healthConfig := &HealthServiceConfig{
				AppSystemCode:          "system-code",
				AppName:                "app-name",
				Description:            "description",
				SmartlogicModel:        "testModel",
				SmartlogicModelConcept: "testConcept",
				SuccessCacheTime:       1 * time.Minute,
			}
			healthService, err := NewHealthService(d.mockService, healthConfig)
			if err != nil {
				t.Fatal(err)
			}
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

func TestConcurrentNotify(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name                           string
		notificationTimes              []string
		duration                       time.Duration
		expectedChangedConceptListDate string
		expectedKafkaMsgCount          int
	}{
		{
			name: "single request",
			notificationTimes: []string{
				"13:00:04.000Z",
			},
			duration:                       50 * time.Millisecond,
			expectedChangedConceptListDate: "13:00:03.990Z",
			expectedKafkaMsgCount:          1,
		},
		{
			name: "10 requests",
			notificationTimes: []string{
				"13:00:00.000Z",
				"13:00:01.000Z",
				"13:00:02.000Z",
				"13:00:03.000Z",
				"13:00:04.000Z",
				"13:00:05.000Z",
				"13:00:06.000Z",
				"13:00:07.000Z",
				"13:00:08.000Z",
				"13:00:09.000Z",
			},
			expectedChangedConceptListDate: "12:59:59.990Z",
			duration:                       50 * time.Millisecond,
			expectedKafkaMsgCount:          1,
		},
	}

	today := time.Now().Format("2006-01-02T")
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			kc := &mockKafkaClient{}
			sl := &mockSmartlogicClient{
				concepts: map[string]string{
					"uuid1": "concept1",
					"uuid2": "concept2",
				},
				getChangedConceptListFunc: func(changeDate time.Time) ([]string, error) {
					assert.Equal(t, test.expectedChangedConceptListDate, changeDate.Format("15:04:05.000Z"))

					return []string{"uuid2"}, nil
				},
			}

			service := NewNotifierService(kc, sl)

			// making sure the ticker will fire at least once during the test execution
			tickerInterval := test.duration / 2
			tk := &ticker{ticker: time.NewTicker(tickerInterval)}

			handler := NewNotifierHandler(service, smartlogicModel, WithTicker(tk))

			m := mux.NewRouter()
			handler.RegisterEndpoints(m)

			var reqs []*http.Request
			for _, tPoint := range test.notificationTimes {
				url := fmt.Sprintf("/notify?affectedGraphId=%s&modifiedGraphId=%s&lastChangeDate=%s%s", smartlogicModel, smartlogicModel, today, tPoint)
				req, _ := http.NewRequest("GET", url, nil)
				reqs = append(reqs, req)
			}

			for _, req := range reqs {
				go func(r *http.Request) {
					start := time.Now()
					recorder := httptest.NewRecorder()
					m.ServeHTTP(recorder, r)
					end := time.Now()

					assert.WithinDuration(t, start, end, tickerInterval)
				}(req)
			}

			time.Sleep(test.duration)

			assert.Equal(t, test.expectedKafkaMsgCount, kc.getSentCount())
		})
	}
}

func TestProcessingNotifyRequestsDoesNotBlock(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name              string
		notificationTimes []string
		duration          time.Duration
		slClient          *mockSmartlogicClient
		expectedTicks     int
	}{
		{
			name: "no requests",
			slClient: &mockSmartlogicClient{
				concepts: map[string]string{
					"uuid1": "concept1",
					"uuid2": "concept2",
				},
				getChangedConceptListFunc: func(changeDate time.Time) ([]string, error) {
					return []string{"uuid2"}, nil
				},
			},
			duration:      100 * time.Millisecond,
			expectedTicks: 10,
		},
		{
			name: "single request",
			slClient: &mockSmartlogicClient{
				concepts: map[string]string{
					"uuid1": "concept1",
					"uuid2": "concept2",
				},
				getChangedConceptListFunc: func(changeDate time.Time) ([]string, error) {
					return []string{"uuid2"}, nil
				},
			},
			notificationTimes: []string{
				"13:00:04.000Z",
			},
			duration:      100 * time.Millisecond,
			expectedTicks: 10,
		},
		{
			name: "10 requests",
			slClient: &mockSmartlogicClient{
				concepts: map[string]string{
					"uuid1": "concept1",
					"uuid2": "concept2",
				},
				getChangedConceptListFunc: func(changeDate time.Time) ([]string, error) {
					return []string{"uuid2"}, nil
				},
			},
			notificationTimes: []string{
				"13:00:00.000Z",
				"13:00:01.000Z",
				"13:00:02.000Z",
				"13:00:03.000Z",
				"13:00:04.000Z",
				"13:00:05.000Z",
				"13:00:06.000Z",
				"13:00:07.000Z",
				"13:00:08.000Z",
				"13:00:09.000Z",
			},
			duration:      1000 * time.Millisecond,
			expectedTicks: 10,
		},
		{
			name: "5 requests with errors on get changed concepts list",
			slClient: &mockSmartlogicClient{
				concepts: nil,
				getChangedConceptListFunc: func(changeDate time.Time) ([]string, error) {
					return nil, errors.New("could not get changed concepts list")
				},
			},
			notificationTimes: []string{
				"13:00:03.000Z",
				"13:00:04.000Z",
				"13:00:05.000Z",
				"13:00:06.000Z",
				"13:00:07.000Z",
			},
			duration:      1000 * time.Millisecond,
			expectedTicks: 10,
		},
	}

	today := time.Now().Format("2017-05-31T")
	for _, test := range testCases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			kc := &mockKafkaClient{}
			service := NewNotifierService(kc, test.slClient)

			tickerInterval := test.duration / 10
			tk := &mockTicker{ticker: time.NewTicker(tickerInterval)}

			handler := NewNotifierHandler(service, smartlogicModel, WithTicker(tk))

			m := mux.NewRouter()
			handler.RegisterEndpoints(m)

			var reqs []*http.Request
			for _, tPoint := range test.notificationTimes {
				url := fmt.Sprintf("/notify?affectedGraphId=%s&modifiedGraphId=%s&lastChangeDate=%s%s", smartlogicModel, smartlogicModel, today, tPoint)
				req, _ := http.NewRequest("GET", url, nil)
				reqs = append(reqs, req)
			}

			for _, req := range reqs {
				go func(r *http.Request) {
					start := time.Now()
					recorder := httptest.NewRecorder()
					m.ServeHTTP(recorder, r)
					end := time.Now()

					assert.WithinDuration(t, start, end, tickerInterval)
				}(req)
			}

			time.Sleep(test.duration)

			assert.InDelta(t, test.expectedTicks, tk.getTicks(), 1)
		})
	}
}

func TestGettingSmartlogicChangesOneRequestAtATime(t *testing.T) {
	t.Parallel()
	log.SetOutput(ioutil.Discard)

	testCases := []struct {
		name     string
		reqCount int
		duration time.Duration
	}{
		{
			name:     "100 notify requests",
			reqCount: 100,
			duration: 2500 * time.Millisecond,
		},
	}
	today := time.Now().Format(TimeFormat)
	requestURI := fmt.Sprintf("/notify?affectedGraphId=%s&modifiedGraphId=%s&lastChangeDate=%s", smartlogicModel, smartlogicModel, today)
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
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

			tickerInterval := test.duration / 5
			tk := &ticker{ticker: time.NewTicker(tickerInterval)}

			handler := NewNotifierHandler(service, smartlogicModel, WithTicker(tk))

			m := mux.NewRouter()
			handler.RegisterEndpoints(m)

			for i := 0; i < test.reqCount; i++ {
				go func() {
					r, _ := http.NewRequest("GET", requestURI, nil)
					start := time.Now()
					recorder := httptest.NewRecorder()
					m.ServeHTTP(recorder, r)
					end := time.Now()

					assert.WithinDuration(t, start, end, tickerInterval)
				}()
			}

			time.Sleep(test.duration)

			assert.Equal(t, 1, sl.getChangedConceptListCallCount())
		})
	}
}
