package smartlogic

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/url"
	"testing"

	"time"

	"sort"

	"github.com/stretchr/testify/assert"
)

func NewSmartlogicTestClient(httpClient httpClient, baseURL string, model string, apiKey string, conceptUriPrefix string) (Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return Client{}, err
	}

	client := Client{
		baseURL:          *u,
		model:            model,
		conceptUriPrefix: conceptUriPrefix,
		apiKey:           apiKey,
		httpClient:       httpClient,
	}

	return client, nil
}

func TestNewSmartlogicClient_Success(t *testing.T) {

	tokenResponseValue := "1234567890"
	tokenResponseString := "{\"access_token\": \"" + tokenResponseValue + "\"}"

	sl, err := NewSmartlogicClient(
		&mockHttpClient{
			resp:       tokenResponseString,
			statusCode: 200,
			err:        nil,
		}, "http://base/url", "modelName", "apiKey", "conceptUriPrefix",
	)
	assert.NoError(t, err)
	assert.EqualValues(t, tokenResponseValue, sl.AccessToken())
}

func TestNewSmartlogicClient_BadURL(t *testing.T) {

	tokenResponseValue := "1234567890"
	tokenResponseString := "{\"access_token\": \"" + tokenResponseValue + "\"}"

	_, err := NewSmartlogicClient(
		&mockHttpClient{
			resp:       tokenResponseString,
			statusCode: 200,
			err:        nil,
		}, "http:// base/url", "modelName", "apiKey", "conceptUriPrefix",
	)
	assert.Error(t, err)
	assert.EqualValues(t, "parse http:// base/url: invalid character \" \" in host name", err.Error())
}

func TestNewSmartlogicClient_NoToken(t *testing.T) {

	tokenResponseString := "{\"1\":1}"

	sl, err := NewSmartlogicClient(
		&mockHttpClient{
			resp:       tokenResponseString,
			statusCode: 200,
			err:        nil,
		}, "http://base/url", "modelName", "apiKey", "conceptUriPrefix",
	)
	assert.NoError(t, err)
	assert.EqualValues(t, "", sl.AccessToken())
}

func TestNewSmartlogicClient_BadResponse(t *testing.T) {

	responseError := errors.New("Errorfield")
	tokenResponseString := "{\"1\":1}"

	_, err := NewSmartlogicClient(
		&mockHttpClient{
			resp:       tokenResponseString,
			statusCode: 404,
			err:        responseError,
		}, "http://base/url", "modelName", "apiKey", "conceptUriPrefix",
	)
	assert.Error(t, err)
	assert.EqualValues(t, responseError, err)
}

func TestNewSmartlogicClient_BadJSON(t *testing.T) {

	tokenResponseString := "{\"1\":}"

	_, err := NewSmartlogicClient(
		&mockHttpClient{
			resp:       tokenResponseString,
			statusCode: 200,
			err:        nil,
		}, "http://base/url", "modelName", "apiKey", "conceptUriPrefix",
	)
	assert.Error(t, err)
	assert.IsType(t, &json.SyntaxError{}, err)
}

func TestClient_MakeRequest_Success(t *testing.T) {
	sl, err := NewSmartlogicTestClient(
		&mockHttpClient{
			resp:       "response",
			statusCode: 200,
			err:        nil,
		}, "http://base/url", "modelName", "apiKey", "conceptUriPrefix",
	)

	resp, err := sl.makeRequest("GET", "http://a/url")
	assert.NoError(t, err)

	defer resp.Body.Close()
	s, err := ioutil.ReadAll(resp.Body)
	assert.EqualValues(t, "response", string(s))
}

func TestClient_MakeRequest_Unauthorized(t *testing.T) {
	sl, err := NewSmartlogicTestClient(
		&mockHttpClient{
			resp:       "response",
			statusCode: 401,
			err:        nil,
		}, "http://base/url", "modelName", "apiKey", "conceptUriPrefix",
	)

	_, err = sl.makeRequest("GET", "http://a/url")
	assert.Error(t, err)
	assert.EqualValues(t, errors.New("Failed to get a valid access token"), err)
}

func TestClient_MakeRequest_DoError(t *testing.T) {
	sl, err := NewSmartlogicTestClient(
		&mockHttpClient{
			resp:       "response",
			statusCode: 200,
			err:        errors.New("Errorfield"),
		}, "http://base/url", "modelName", "apiKey", "conceptUriPrefix",
	)

	_, err = sl.makeRequest("GET", "http://a/url")
	assert.Error(t, err)
	assert.EqualValues(t, errors.New("Errorfield"), err)
}

func TestClient_MakeRequest_RequestError(t *testing.T) {
	sl, err := NewSmartlogicTestClient(
		&mockHttpClient{
			resp:       "response",
			statusCode: 200,
			err:        nil,
		}, "http://base/url", "modelName", "apiKey", "conceptUriPrefix",
	)

	_, err = sl.makeRequest("GET", "http:// a/url")
	assert.Error(t, err)
	assert.EqualValues(t, "parse http:// a/url: invalid character \" \" in host name", err.Error())
}

func TestClient_GetConcept_URLError(t *testing.T) {
	conceptResponse := "response"

	sl, err := NewSmartlogicTestClient(
		&mockHttpClient{
			resp:       conceptResponse,
			statusCode: 200,
			err:        nil,
		}, "http://base/url", "modelName", "apiKey", "conceptUriPrefix",
	)
	assert.NoError(t, err)

	concept, err := sl.GetConcept("a-uuid")
	assert.NoError(t, err)
	assert.EqualValues(t, conceptResponse, concept)
}

func TestClient_GetChangedConceptList_Success(t *testing.T) {
	conceptResponse, err := ioutil.ReadFile("../resources/get-changed-concepts.json")

	sl, err := NewSmartlogicTestClient(
		&mockHttpClient{
			resp:       string(conceptResponse),
			statusCode: 200,
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
	conceptResponse, err := ioutil.ReadFile("../resources/get-changed-concepts.json")

	requestError := errors.New("anerror")

	sl, err := NewSmartlogicTestClient(
		&mockHttpClient{
			resp:       string(conceptResponse),
			statusCode: 200,
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
		&mockHttpClient{
			resp:       string(conceptResponse),
			statusCode: 200,
			err:        nil,
		}, "http://base/url", "modelName", "apiKey", "conceptUriPrefix",
	)
	assert.NoError(t, err)

	response, err := sl.GetChangedConceptList(time.Now())
	assert.Error(t, err)
	assert.IsType(t, &json.SyntaxError{}, err)
	assert.Empty(t, response)
}
