package smartlogic

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

const slTokenURL = "https://cloud.smartlogic.com/token"
const maxAccessFailureCount = 5
const thingURIPrefix = "http://www.ft.com/thing/"
const managedLocationURIPrefix = "http://www.ft.com/ontology/managedlocation/"

type httpClient interface {
	Do(req *http.Request) (resp *http.Response, err error)
}

type Clienter interface {
	GetConcept(uuid string) ([]byte, error)
	GetChangedConceptList(changeDate time.Time) ([]string, error)
	AccessToken() string
}

type Client struct {
	baseURL            url.URL
	model              string
	conceptUriPrefix   string
	apiKey             string
	httpClient         httpClient
	accessToken        string
	accessFailureCount int
}

func NewSmartlogicClient(httpClient httpClient, baseURL string, model string, apiKey string, conceptUriPrefix string) (Clienter, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return &Client{}, err
	}

	client := Client{
		baseURL:          *u,
		model:            model,
		conceptUriPrefix: conceptUriPrefix,
		apiKey:           apiKey,
		httpClient:       httpClient,
	}

	err = client.GenerateToken()
	if err != nil {
		return &Client{}, err
	}
	return &client, nil
}

func (c *Client) AccessToken() string {
	return c.accessToken
}

func (c *Client) GetConcept(uuid string) ([]byte, error) {
	reqURL := c.baseURL
	q := "path=" + c.buildConceptPath(uuid)
	reqURL.RawQuery = q

	log.Debugf("Smartlogic Request URL: %v", reqURL.String())
	resp, err := c.makeRequest("GET", reqURL.String())
	if err != nil {
		log.WithError(err).WithField("method", "GetConcept").Error("Error creating the request")
		return []byte{}, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.WithError(err).WithField("method", "GetConcept").Error("Error reading the response body")
		return []byte{}, err
	}

	return body, nil
}

func (c *Client) GetChangedConceptList(changeDate time.Time) ([]string, error) {
	// path=tchmodel:FTSemanticPlayground/changes&since=2017-05-31T13:00:00.000Z&properties=[]
	reqURL := c.baseURL
	q := `path=tchmodel:` + c.model + `/changes&since=` + changeDate.Format("2006-01-02T15:04:05.000Z") + `&properties=%5B%5D`
	reqURL.RawQuery = q

	log.Debugf("Smartlogic Change List Request URL: %v", reqURL.String())
	resp, err := c.makeRequest("GET", reqURL.String())
	if err != nil {
		log.WithError(err).WithField("method", "GetChangedConceptList").Error("Error creating the request")
		return nil, err
	}

	var graph Graph
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&graph)
	if err != nil {
		log.WithError(err).WithField("method", "GetChangedConceptList").Error("Error decoding the response body")
		return nil, err
	}

	changedURIs := map[string]bool{}
	for _, changeset := range graph.Changesets {
		for _, v := range changeset.Concepts {
			changedURIs[v.URI] = true
		}
	}

	output := []string{}
	for k := range changedURIs {
		if uuid, ok := getUUIDfromValidURI(k); ok {
			output = append(output, uuid)
		}
	}
	return output, nil
}

func getUUIDfromValidURI(uri string) (string, bool) {
	if !strings.Contains(uri, "ConceptScheme") {
		if strings.HasPrefix(uri, thingURIPrefix) {
			return strings.TrimPrefix(uri, thingURIPrefix), true
		}
		if strings.HasPrefix(uri, managedLocationURIPrefix) {
			return strings.TrimPrefix(uri, managedLocationURIPrefix), true
		}
	}
	return "", false
}

func (c *Client) makeRequest(method, url string) (*http.Response, error) {
	if c.accessFailureCount >= maxAccessFailureCount {
		// We've failed to get a valid access token multiple times in a row, so just error out.
		log.WithField("method", "makeRequest").Error("Failed to get a valid access token")
		return nil, errors.New("Failed to get a valid access token")
	}

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		log.WithError(err).WithField("method", "makeRequest").Error("Error creating the request")
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.WithError(err).WithField("method", "makeRequest").Error("Error making the request")
		return resp, err
	}

	// We're checking if we got a 401, which would be because the token had expired.  If it has, generate a new
	// one and then make the request again.
	if resp.StatusCode == 401 {
		resp.Body.Close()
		c.accessFailureCount++
		c.GenerateToken()
		return c.makeRequest(method, url)
	}
	c.accessFailureCount = 0
	return resp, err
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	UserName    string `json:"userName"`
	Issued      string `json:".issued"`
	Expires     string `json:".expires"`
}

// Tokens have a limited life, so to be safe we should generate a new one for each notification received.
func (c *Client) GenerateToken() error {
	data := url.Values{}
	data.Set("grant_type", "apikey")
	data.Set("key", c.apiKey)

	req, err := http.NewRequest("POST", slTokenURL, bytes.NewBufferString(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err != nil {
		log.WithError(err).WithField("method", "GenerateToken").Error("Error creating the request")
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.WithError(err).WithField("method", "GenerateToken").Error("Error making the request")
		return err
	}

	defer resp.Body.Close()

	var tokenResponse TokenResponse
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&tokenResponse)
	if err != nil {
		log.WithError(err).WithField("method", "GenerateToken").Error("Error decoding the response body")
		return err
	}
	log.Debug("Setting Smartlogic access token")
	c.accessToken = tokenResponse.AccessToken
	return nil
}

func (c *Client) buildConceptPath(uuid string) string {
	/*
		Because the API call needs to be made as part of the 'path' query parameter, we need to escape the IRI twice,
		once to encode the IRI according to how Smartlogic needs it and once to encode it as a query parameter.
	*/
	concept := "<" + c.conceptUriPrefix + uuid + ">"
	encodedConcept := url.QueryEscape(url.QueryEscape(concept))

	encodedProperties := url.QueryEscape("<http://www.ft.com/ontology/shortLabel>")
	return "model:" + c.model + "/" + encodedConcept + "&properties=%5B%5D,skosxl:prefLabel/skosxl:literalForm,skosxl:altLabel/skosxl:literalForm," + encodedProperties + "/skosxl:literalForm"
}
