package smartlogic

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	slGetCredentialsURL = "https://cloud.smartlogic.com/token"
	slTimeFormat        = "2006-01-02T15:04:05.000Z"

	maxAccessFailureCount = 5

	thingURIPrefix           = "http://www.ft.com/thing/"
	managedLocationURIPrefix = "http://www.ft.com/ontology/managedlocation/"
)

var ErrorConceptDoesNotExist = errors.New("concept does not exist")

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
	conceptURIPrefix   string
	apiKey             string
	httpClient         httpClient
	accessToken        string
	accessFailureCount int
}

func NewSmartlogicClient(httpClient httpClient, baseURL string, model string, apiKey string, conceptURIPrefix string) (Clienter, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return &Client{}, err
	}

	client := Client{
		baseURL:          *u,
		model:            model,
		conceptURIPrefix: conceptURIPrefix,
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

// GetConcept returns the json-ld Smartlogic representation of a concept with the given uuid via calling the Smartlogic API.
func (c *Client) GetConcept(uuid string) ([]byte, error) {
	reqURL := c.baseURL
	q := "path=" + c.buildConceptPath(uuid)
	reqURL.RawQuery = q

	entry := log.WithField("method", "GetConcept").WithField("uuid", uuid)
	entry.Debugf("Smartlogic Request URL: %v", reqURL.String())

	resp, err := c.makeRequest("GET", reqURL.String())
	if err != nil {
		entry.WithError(err).Error("Error creating the request")
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("smartlogic returned status %v getting concept with uuid %v", resp.StatusCode, uuid)
		entry.WithError(err).Error("Error response returned")
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		entry.WithError(err).Error("Error reading the response body")
		return nil, err
	}
	// As Smartlogic returns 200 response for non-existing concept with simple representation of the non existing concept,
	// we additionally validate the response in order to check whether the response is for existing concept.
	ok, err := c.isExistingConcept(body)
	if err != nil {
		errorMsg := fmt.Errorf("invalid concept representation returned for uuid %v", uuid)
		entry.WithError(err).WithField("body", string(body)).Error(errorMsg)
		return nil, errorMsg
	}
	if !ok {
		return nil, ErrorConceptDoesNotExist
	}
	return body, nil
}

// isExistingConcept checks whether the Smartlogic json-ld concept response is for existing concept
func (c *Client) isExistingConcept(conceptResp []byte) (bool, error) {
	response := struct {
		Graph []struct {
			SemGUUID []struct {
				Value string `json:"@value"`
			} `json:"sem:guid"`
		} `json:"@graph"`
	}{}
	err := json.Unmarshal(conceptResp, &response)
	if err != nil {
		return false, fmt.Errorf("failed to parse Smartlogic response: %w", err)
	}
	if len(response.Graph) == 0 {
		return false, errors.New("invalid Smartlogic concept response")
	}
	// the json-ld representation of existing concept has "sem:guid" value, as oppose to the json-ld representation of
	// non-existing concept that only has @id property
	if len(response.Graph[0].SemGUUID) == 0 || response.Graph[0].SemGUUID[0].Value == "" {
		return false, nil
	}
	return true, nil
}

// GetChangedConceptList returns a list of uuids of concepts that were changed since specified time.
func (c *Client) GetChangedConceptList(changeDate time.Time) ([]string, error) {
	reqURL := c.baseURL
	reqURL.RawQuery = c.buildChangesAPIQueryParams(changeDate).Encode()

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
		return nil, errors.New("failed to get a valid access token")
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
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		c.accessFailureCount++
		err = c.GenerateToken()
		if err != nil {
			// we were not able to generate new token, we will log it and try again to make the request
			// which will try again to generate new token
			log.Infof("Failed to generate new Smartlogic token: %v", err)
		}
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

	req, err := http.NewRequest("POST", slGetCredentialsURL, bytes.NewBufferString(data.Encode()))
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
	concept := "<" + c.conceptURIPrefix + uuid + ">"
	encodedConcept := url.QueryEscape(url.QueryEscape(concept))

	encodedProperties := url.QueryEscape("<http://www.ft.com/ontology/shortLabel>")
	return "model:" + c.model + "/" + encodedConcept + "&properties=%5B%5D,skosxl:prefLabel/skosxl:literalForm,skosxl:altLabel/skosxl:literalForm," + encodedProperties + "/skosxl:literalForm"
}

// buildChangesAPIQueryParams returns map of type url.Values containing all query params needed to perform request to the Smartlogic API
// that returns the changes on the model since specified time
func (c *Client) buildChangesAPIQueryParams(changeDate time.Time) url.Values {
	// Construct the request query params in such way that only the ids of the concepts affected by the change will be returned.
	// Example: path=tchmodel:MODEL_ID/teamwork:Change/rdf:instance&properties=sem:about&filters=subject(sem:committed%3E%222020-04-05T00:00:00.990Z%22%5E%5Exsd:dateTime)
	// URL decoded example: path=tchmodel:MODEL_ID/teamwork:Change/rdf:instance&properties=sem:about&filters=subject(sem:committed>"2020-04-05T00:00:00.990Z"^^xsd:dateTime)
	queryParams := url.Values{}

	queryParams.Add("path", fmt.Sprintf("tchmodel:%s/teamwork:Change/rdf:instance", c.model))
	queryParams.Add("properties", "sem:about")

	timeFilter := fmt.Sprintf("sem:committed>\"%s\"^^xsd:dateTime", changeDate.Format(slTimeFormat))
	queryParams.Add("filters", fmt.Sprintf("subject(%s)", timeFilter))

	return queryParams
}
