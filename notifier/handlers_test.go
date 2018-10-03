package notifier

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

const ExpectedContentType = "application/json"

func TestHandlers(t *testing.T) {
	testCases := []struct {
		name        string
		method      string
		url         string
		requestBody string
		resultCode  int
		resultBody  string
		err         error
		concepts    map[string]string
		changes     []string
	}{
		{
			"Notify - Success",
			"GET",
			"/notify?affectedGraphId=1&modifiedGraphId=2&lastChangeDate=2017-05-31T13:00:00.000Z",
			"",
			200,
			"{\"message\": \"Messages successfully sent to Kafka\"}",
			nil,
			map[string]string{},
			[]string{},
		},
		{
			"Notify - Missing query parameters",
			"GET",
			"/notify?modifiedGraphId=2&lastChangeDate=2017-05-31T13:00:00.000Z",
			"",
			400,
			"{\"message\": \"Query parameters were not set: affectedGraphId\"}",
			nil,
			map[string]string{},
			[]string{},
		},
		{
			"Notify - Missing all query parameters",
			"GET",
			"/notify",
			"",
			400,
			"{\"message\": \"Query parameters were not set: modifiedGraphId, affectedGraphId, lastChangeDate\"}",
			nil,
			map[string]string{},
			[]string{},
		},
		{
			"Notify - Bad query parameters ",
			"GET",
			"/notify?affectedGraphId=1&modifiedGraphId=2&lastChangeDate=notadate",
			"",
			400,
			"{\"message\": \"Date is not in the format 2006-01-02T15:04:05.000Z\"}",
			nil,
			map[string]string{},
			[]string{},
		},
		{
			"Notify - Error",
			"GET",
			"/notify?affectedGraphId=1&modifiedGraphId=2&lastChangeDate=2017-05-31T13:00:00.000Z",
			"",
			500,
			"{\"message\": \"There was an error completing the notify\", \"error\": \"anerror\"}",
			errors.New("anerror"),
			map[string]string{},
			[]string{},
		},
		{
			"Force Notify - Success",
			"POST",
			"/force-notify",
			`{"uuids": ["1","2","3"]}`,
			200,
			"Concept notification completed",
			nil,
			map[string]string{
				"1": "1",
				"2": "2",
				"3": "3",
			},
			[]string{},
		},
		{
			"Force Notify - Bad Payload",
			"POST",
			"/force-notify",
			`{"uuids": "1","2","3"]}`,
			400,
			"{\"message\": \"There was an error decoding the payload\", \"error\": \"invalid character ',' after object key\"}",
			nil,
			map[string]string{
				"1": "1",
				"2": "2",
				"3": "3",
			},
			[]string{},
		},
		{
			"Get Concept - Success",
			"GET",
			"/concept/1",
			``,
			200,
			"1",
			nil,
			map[string]string{
				"1": "1",
				"2": "2",
				"3": "3",
			},
			[]string{},
		},
		{
			"Get Concept - Error",
			"GET",
			"/concept/11",
			``,
			500,
			"{\"message\": \"There was an error retrieving the concept\", \"error\": \"Can't find concept\"}",
			nil,
			map[string]string{
				"1": "1",
				"2": "2",
				"3": "3",
			},
			[]string{},
		},
		{
			"__health",
			"GET",
			"/__health",
			``,
			200,
			"IGNORE",
			errors.New("anerror"),
			map[string]string{
				"1": "1",
				"2": "2",
				"3": "3",
			},
			[]string{},
		},
		{
			"__build-info",
			"GET",
			"/__build-info",
			``,
			200,
			"IGNORE",
			errors.New("anerror"),
			map[string]string{
				"1": "1",
				"2": "2",
				"3": "3",
			},
			[]string{},
		},
		{
			"__gtg",
			"GET",
			"/__gtg",
			``,
			503,
			"IGNORE",
			nil,
			map[string]string{
				"1": "1",
				"2": "2",
				"3": "3",
			},
			[]string{},
		},
	}

	for _, d := range testCases {
		t.Run(d.name, func(t *testing.T) {
			mockService := NewMockService(d.concepts, d.changes, d.err)
			handler := NewNotifierHandler(mockService)
			m := mux.NewRouter()
			handler.RegisterEndpoints(m)
			handler.RegisterAdminEndpoints(m, "system-code", "app-name", "description", time.Second)

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

func TestHealthCheckError(t *testing.T) {
	changes := []string{}
	concepts := map[string]string{
		"healthcheck-concept": "1",
	}
	mockSvc := NewMockService(concepts, changes, errors.New("something bad"))
	handler := NewNotifierHandler(mockSvc)
	m := mux.NewRouter()
	handler.RegisterEndpoints(m)
	handler.RegisterAdminEndpoints(m, "system-code", "app-name", "description", time.Second)

	req, _ := http.NewRequest("GET", "/__gtg", bytes.NewBufferString(""))
	rr := httptest.NewRecorder()
	m.ServeHTTP(rr, req)

	b, err := ioutil.ReadAll(rr.Body)
	assert.NoError(t, err)
	body := string(b)
	assert.Equal(t, 503, rr.Code, "__gtg")
	assert.Equal(t, "something bad", body, "__gtg")
}

func TestHealthCheckCache(t *testing.T) {
	changes := []string{}
	concepts := map[string]string{
		"b1a492d9-dcfe-43f8-8072-17b4618a78fd": "1",
	}
	mockSvc := NewMockService(concepts, changes, nil)
	handler := NewNotifierHandler(mockSvc)
	m := mux.NewRouter()
	handler.RegisterEndpoints(m)
	handler.RegisterAdminEndpoints(m, "system-code", "app-name", "description", time.Second)

	// check that gtg returns ok
	{
		req, _ := http.NewRequest("GET", "/__gtg", bytes.NewBufferString(""))
		rr := httptest.NewRecorder()
		m.ServeHTTP(rr, req)
		b, err := ioutil.ReadAll(rr.Body)
		assert.NoError(t, err)
		body := string(b)
		assert.Equal(t, 200, rr.Code, "__gtg")
		assert.Equal(t, "OK", body, "__gtg")
	}

	// tell mock to return error
	mockSvc.(*mockService).err = errors.New("something random")

	// but expect gtg to return cached ok
	{
		req, _ := http.NewRequest("GET", "/__gtg", bytes.NewBufferString(""))
		rr := httptest.NewRecorder()
		m.ServeHTTP(rr, req)
		b, err := ioutil.ReadAll(rr.Body)
		assert.NoError(t, err)
		body := string(b)
		assert.Equal(t, 200, rr.Code, "__gtg")
		assert.Equal(t, "OK", body, "__gtg")
	}

	// wait for cache to clear
	time.Sleep(time.Second)

	// and expect gtg to return err
	{
		req, _ := http.NewRequest("GET", "/__gtg", bytes.NewBufferString(""))
		rr := httptest.NewRecorder()
		m.ServeHTTP(rr, req)
		b, err := ioutil.ReadAll(rr.Body)
		assert.NoError(t, err)
		body := string(b)
		assert.Equal(t, 503, rr.Code, "__gtg")
		assert.Equal(t, "something random", body, "__gtg")
	}

	// tell mock to return ok
	mockSvc.(*mockService).err = nil

	// and expect gtg to return ok instantly as we don't cache failure
	{
		req, _ := http.NewRequest("GET", "/__gtg", bytes.NewBufferString(""))
		rr := httptest.NewRecorder()
		m.ServeHTTP(rr, req)
		b, err := ioutil.ReadAll(rr.Body)
		assert.NoError(t, err)
		body := string(b)
		assert.Equal(t, 200, rr.Code, "__gtg")
		assert.Equal(t, "OK", body, "__gtg")
	}
}

func newRequest(method, url string, body string) *http.Request {
	var payload io.Reader
	if body != "" {
		payload = bytes.NewReader([]byte(body))
	}
	req, err := http.NewRequest(method, url, payload)
	req.Header = map[string][]string{
		"Content-Type": {ExpectedContentType},
	}
	if err != nil {
		panic(err)
	}
	return req
}
