package smartlogic

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"sort"
	"testing"
	"time"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/stretchr/testify/assert"
)

func NewSmartlogicTestClient(httpClient httpClient, baseURL string, model string, apiKey string, conceptURIPrefix string) (Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return Client{}, err
	}

	client := Client{
		baseURL:          *u,
		model:            model,
		conceptURIPrefix: conceptURIPrefix,
		apiKey:           apiKey,
		httpClient:       httpClient,
		log:              logger.NewUnstructuredLogger(),
	}

	return client, nil
}

func TestNewSmartlogicClient_Success(t *testing.T) {

	tokenResponseValue := "1234567890"
	tokenResponseString := "{\"access_token\": \"" + tokenResponseValue + "\"}"

	sl, err := NewSmartlogicClient(
		&mockHTTPClient{
			resp:       tokenResponseString,
			statusCode: http.StatusOK,
			err:        nil,
		}, "http://base/url", "modelName", "apiKey", "conceptUriPrefix",
		logger.NewUnstructuredLogger(),
	)
	assert.NoError(t, err)
	assert.EqualValues(t, tokenResponseValue, sl.AccessToken())
}

func TestNewSmartlogicClient_BadURL(t *testing.T) {

	tokenResponseValue := "1234567890"
	tokenResponseString := "{\"access_token\": \"" + tokenResponseValue + "\"}"

	_, err := NewSmartlogicClient(
		&mockHTTPClient{
			resp:       tokenResponseString,
			statusCode: http.StatusOK,
			err:        nil,
		}, "http:// base/url", "modelName", "apiKey", "conceptUriPrefix",
		logger.NewUnstructuredLogger(),
	)
	assert.Error(t, err)
}

func TestNewSmartlogicClient_NoToken(t *testing.T) {

	tokenResponseString := "{\"1\":1}"

	sl, err := NewSmartlogicClient(
		&mockHTTPClient{
			resp:       tokenResponseString,
			statusCode: http.StatusOK,
			err:        nil,
		}, "http://base/url", "modelName", "apiKey", "conceptUriPrefix",
		logger.NewUnstructuredLogger(),
	)
	assert.NoError(t, err)
	assert.EqualValues(t, "", sl.AccessToken())
}

func TestNewSmartlogicClient_BadResponse(t *testing.T) {

	responseError := errors.New("Errorfield")
	tokenResponseString := "{\"1\":1}"

	_, err := NewSmartlogicClient(
		&mockHTTPClient{
			resp:       tokenResponseString,
			statusCode: http.StatusNotFound,
			err:        responseError,
		}, "http://base/url", "modelName", "apiKey", "conceptUriPrefix",
		logger.NewUnstructuredLogger(),
	)
	assert.Error(t, err)
	assert.EqualValues(t, responseError, err)
}

func TestNewSmartlogicClient_BadJSON(t *testing.T) {

	tokenResponseString := "{\"1\":}"

	_, err := NewSmartlogicClient(
		&mockHTTPClient{
			resp:       tokenResponseString,
			statusCode: http.StatusOK,
			err:        nil,
		}, "http://base/url", "modelName", "apiKey", "conceptUriPrefix",
		logger.NewUnstructuredLogger(),
	)
	assert.Error(t, err)
	assert.IsType(t, &json.SyntaxError{}, err)
}

func TestClient_MakeRequest_Success(t *testing.T) {
	sl, err := NewSmartlogicTestClient(
		&mockHTTPClient{
			resp:       "response",
			statusCode: http.StatusOK,
			err:        nil,
		}, "http://base/url", "modelName", "apiKey", "conceptUriPrefix",
	)
	assert.NoError(t, err)

	resp, err := sl.makeRequest("GET", "http://a/url")
	assert.NoError(t, err)

	defer resp.Body.Close()
	s, err := ioutil.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.EqualValues(t, "response", string(s))
}

func TestClient_MakeRequest_Unauthorized(t *testing.T) {
	sl, err := NewSmartlogicTestClient(
		&mockHTTPClient{
			resp:       "response",
			statusCode: http.StatusUnauthorized,
			err:        nil,
		}, "http://base/url", "modelName", "apiKey", "conceptUriPrefix",
	)
	assert.NoError(t, err)

	_, err = sl.makeRequest("GET", "http://a/url")
	assert.Error(t, err)
	assert.EqualValues(t, errors.New("failed to get a valid access token"), err)
}

func TestClient_MakeRequest_DoError(t *testing.T) {
	sl, err := NewSmartlogicTestClient(
		&mockHTTPClient{
			resp:       "response",
			statusCode: http.StatusOK,
			err:        errors.New("Errorfield"),
		}, "http://base/url", "modelName", "apiKey", "conceptUriPrefix",
	)
	assert.NoError(t, err)

	_, err = sl.makeRequest("GET", "http://a/url")
	assert.Error(t, err)
	assert.EqualValues(t, errors.New("Errorfield"), err)
}

func TestClient_MakeRequest_RequestError(t *testing.T) {
	sl, err := NewSmartlogicTestClient(
		&mockHTTPClient{
			resp:       "response",
			statusCode: http.StatusOK,
			err:        nil,
		}, "http://base/url", "modelName", "apiKey", "conceptUriPrefix",
	)
	assert.NoError(t, err)

	_, err = sl.makeRequest("GET", "http:// a/url")
	assert.Error(t, err)
}

func TestClient_GetConcept(t *testing.T) {
	tests := []struct {
		name          string
		slResponse    string
		slStatus      int
		httpError     error
		expectedError error
	}{
		{
			name:          "success",
			slResponse:    "testdata/ft-concept.json",
			slStatus:      http.StatusOK,
			httpError:     nil,
			expectedError: nil,
		},
		{
			name:          "http failure",
			slResponse:    "testdata/ft-concept.json",
			slStatus:      http.StatusOK,
			httpError:     errors.New("http request failed for some reason"),
			expectedError: errors.New("some error to be returned, exact error is not relevant"),
		},
		{
			name:          "smartlogic non-200 response",
			slResponse:    "testdata/ft-concept.json",
			slStatus:      http.StatusInternalServerError,
			httpError:     nil,
			expectedError: errors.New("some error to be returned, exact error is not relevant"),
		},
		{
			name:          "smartlogic non-existing concept response",
			slResponse:    "testdata/non-existing-concept.json",
			slStatus:      http.StatusOK,
			httpError:     nil,
			expectedError: ErrorConceptDoesNotExist,
		},
		{
			name:          "smartlogic invalid response",
			slResponse:    "testdata/invalid-concept.json",
			slStatus:      http.StatusOK,
			httpError:     nil,
			expectedError: errors.New("some error to be returned, exact error is not relevant"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			slResponse, err := ioutil.ReadFile(test.slResponse)
			assert.NoError(t, err)

			sl, err := NewSmartlogicTestClient(
				&mockHTTPClient{
					resp:       string(slResponse),
					statusCode: test.slStatus,
					err:        test.httpError,
				}, "http://base/url", "modelName", "apiKey", "conceptUriPrefix",
			)
			assert.NoError(t, err)

			concept, err := sl.GetConcept("test-uuid")
			if err == nil && test.expectedError != nil {
				t.Error("expected error getting concept")
			}
			if err != nil && test.expectedError == nil {
				t.Errorf("unexpected error getting concept: %v", err)
			}
			if errors.Is(test.expectedError, ErrorConceptDoesNotExist) &&
				!errors.Is(test.expectedError, ErrorConceptDoesNotExist) {
				t.Errorf("expected ErrorConceptDoesNotExist, got %v", err)
			}
			if test.expectedError == nil {
				assert.Equal(t, concept, slResponse)
			}
		})
	}
}

func TestClient_GetChangedConceptList_Success(t *testing.T) {
	conceptResponse, err := ioutil.ReadFile("testdata/get-changed-concepts.json")
	assert.NoError(t, err)

	sl, err := NewSmartlogicTestClient(
		&mockHTTPClient{
			resp:       string(conceptResponse),
			statusCode: http.StatusOK,
			err:        nil,
		}, "http://base/url", "modelName", "apiKey", "conceptUriPrefix",
	)
	assert.NoError(t, err)

	response, err := sl.GetChangedConceptList(time.Now())
	assert.NoError(t, err)

	expectedResponse := []string{"testTypeMetadata", "fd55c1f0-6c5e-4869-aed4-6816836ffdb9"}

	sort.Strings(expectedResponse)
	sort.Strings(response)
	assert.EqualValues(t, expectedResponse, response)
}

func TestClient_GetChangedConceptList_RequestError(t *testing.T) {
	conceptResponse, err := ioutil.ReadFile("testdata/get-changed-concepts.json")
	assert.NoError(t, err)

	requestError := errors.New("anerror")

	sl, err := NewSmartlogicTestClient(
		&mockHTTPClient{
			resp:       string(conceptResponse),
			statusCode: http.StatusOK,
			err:        requestError,
		}, "http://base/url", "modelName", "apiKey", "conceptUriPrefix",
	)
	assert.NoError(t, err)

	response, err := sl.GetChangedConceptList(time.Now())
	assert.Error(t, err)
	assert.Equal(t, requestError, err)
	assert.Empty(t, response)
}

func TestClient_GetChangedConceptList_BadResponseBody(t *testing.T) {
	conceptResponse := "terrible body"

	sl, err := NewSmartlogicTestClient(
		&mockHTTPClient{
			resp:       conceptResponse,
			statusCode: http.StatusOK,
			err:        nil,
		}, "http://base/url", "modelName", "apiKey", "conceptUriPrefix",
	)
	assert.NoError(t, err)

	response, err := sl.GetChangedConceptList(time.Now())
	assert.Error(t, err)
	assert.IsType(t, &json.SyntaxError{}, err)
	assert.Empty(t, response)
}

func TestClient_buildChangesAPIQueryParams(t *testing.T) {
	changeDate, err := time.Parse(slTimeFormat, "2020-04-27T00:00:00.000Z")
	assert.NoError(t, err)

	client, err := NewSmartlogicTestClient(&mockHTTPClient{}, "http://base/url", "modelName", "apiKey", "conceptUriPrefix")
	assert.NoError(t, err)

	queryParams := client.buildChangesAPIQueryParams(changeDate)
	assert.Contains(t, queryParams, "path")
	assert.Equal(t, queryParams.Get("path"), "tchmodel:modelName/teamwork:Change/rdf:instance")

	assert.Contains(t, queryParams, "properties")
	assert.Equal(t, queryParams.Get("properties"), "sem:about")

	assert.Contains(t, queryParams, "filters")
	assert.Equal(t, queryParams.Get("filters"), "subject(sem:committed>\"2020-04-27T00:00:00.000Z\"^^xsd:dateTime)")
}
