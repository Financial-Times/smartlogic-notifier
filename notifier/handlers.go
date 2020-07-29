package notifier

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Financial-Times/smartlogic-notifier/smartlogic"
	transactionidutils "github.com/Financial-Times/transactionid-utils-go"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

// TimeFormat is the format used to read time values from request parameters
const TimeFormat = "2006-01-02T15:04:05Z"

// maxTimeValue represents the maximum useful time value (for comparisons like finding the minimum value in a range of times)
var maxTimeValue = time.Unix(1<<63-62135596801, 999999999)

// LastChangeLimit represents the upper limit to how far in the past we can reingest smartlogic updates
var LastChangeLimit = time.Hour * 168

type Handler struct {
	notifier  Servicer
	ticker    Ticker
	requestCh chan notificationRequest
	model     string
}

func NewNotifierHandler(notifier Servicer, smartlogicModel string, opts ...func(*Handler)) *Handler {
	h := &Handler{
		notifier:  notifier,
		ticker:    &ticker{ticker: time.NewTicker(5 * time.Second)},
		requestCh: make(chan notificationRequest, 1),
		model:     smartlogicModel,
	}

	for _, opt := range opts {
		opt(h)
	}

	go h.processNotifyRequests()

	return h
}

type Ticker interface {
	Tick()
	Stop()
}

func WithTicker(t Ticker) func(*Handler) {
	return func(h *Handler) {
		h.ticker.Stop()
		h.ticker = t
	}
}

func (h *Handler) HandleNotify(resp http.ResponseWriter, req *http.Request) {
	vars := req.URL.Query()
	notSet := checkQueryParams(h, &vars)
	if len(notSet) > 0 {
		writeJSONResponseMessage(resp, http.StatusBadRequest, responseData{Msg: `Query parameters are missing or incorrect: ` + strings.Join(notSet, ", ")})
		return
	}
	lastChange, err := validateLastChangeDate(vars.Get("lastChangeDate"))
	if err != nil {
		writeJSONResponseMessage(resp, http.StatusBadRequest, responseData{Msg: err.Error()})
		return
	}
	go func() {
		transactionID := req.Header.Get(transactionidutils.TransactionIDHeader)
		h.requestCh <- notificationRequest{
			notifySince:   lastChange,
			transactionID: transactionID,
		}
	}()

	writeJSONResponseMessage(resp, http.StatusOK, responseData{Msg: "Concepts successfully ingested"})
}

func checkQueryParams(h *Handler, vars *url.Values) []string {
	var notSet []string
	modifiedGraphID := vars.Get("modifiedGraphId")
	if modifiedGraphID == "" || modifiedGraphID != h.model {
		notSet = append(notSet, "modifiedGraphId")
	}
	affectedGraphID := vars.Get("affectedGraphId")
	if affectedGraphID == "" || affectedGraphID != h.model {
		notSet = append(notSet, "affectedGraphId")
	}
	lastChangeDate := vars.Get("lastChangeDate")
	if lastChangeDate == "" {
		notSet = append(notSet, "lastChangeDate")
	}
	return notSet
}

func (h *Handler) HandleGetConcepts(resp http.ResponseWriter, req *http.Request) {
	vars := req.URL.Query()
	lastChangeDate := vars.Get("lastChangeDate")
	if lastChangeDate == "" {
		writeJSONResponseMessage(resp, http.StatusBadRequest, responseData{Msg: "Query parameter lastChangeDate was not set."})
		return
	}

	lastChange, err := validateLastChangeDate(lastChangeDate)
	if err != nil {
		writeJSONResponseMessage(resp, http.StatusBadRequest, responseData{Msg: err.Error()})
		return
	}

	uuids, err := h.notifier.GetChangedConceptList(lastChange)
	if err != nil {
		writeJSONResponseMessage(resp, http.StatusInternalServerError, responseData{Msg: "There was an error getting the changes", Err: err})
		return
	}
	uuidsJson, err := json.Marshal(uuids)
	if err != nil {
		writeJSONResponseMessage(resp, http.StatusInternalServerError, responseData{Msg: "There was an error encoding the response", Err: err})
		return
	}
	writeResponseData(resp, http.StatusOK, "application/json", string(uuidsJson))
}

func (h *Handler) HandleForceNotify(resp http.ResponseWriter, req *http.Request) {
	type payload struct {
		UUIDs []string `json:"uuids,omitempty"`
	}
	var pl payload
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&pl)
	if err != nil {
		writeJSONResponseMessage(resp, http.StatusBadRequest, responseData{Msg: "There was an error decoding the payload", Err: err})
		return
	}

	if pl.UUIDs == nil {
		writeJSONResponseMessage(resp, http.StatusBadRequest, responseData{Msg: "No 'uuids' parameter provided"})
		return
	}

	err = h.notifier.ForceNotify(pl.UUIDs, req.Header.Get(transactionidutils.TransactionIDHeader))
	if err != nil {
		writeJSONResponseMessage(resp, http.StatusInternalServerError, responseData{Msg: "There was an error completing the force notify"})
		return
	}
	writeResponseData(resp, http.StatusOK, "text/plain", "Concept notification completed")
}

func (h *Handler) HandleGetConcept(resp http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	uuid, ok := vars["uuid"]

	if !ok {
		writeJSONResponseMessage(resp, http.StatusBadRequest, responseData{Msg: "UUID was not set."})
		return
	}

	concept, err := h.notifier.GetConcept(uuid)
	if err != nil {
		errStatus := http.StatusInternalServerError
		if errors.Is(err, smartlogic.ErrorConceptDoesNotExist) {
			errStatus = http.StatusNotFound
		}
		writeJSONResponseMessage(resp, errStatus, responseData{Msg: "There was an error retrieving the concept", Err: err})
		return
	}
	writeResponseData(resp, http.StatusOK, "application/ld+json", string(concept))
}

func (h *Handler) RegisterEndpoints(router *mux.Router) {
	notifyHandler := handlers.MethodHandler{
		"GET": http.HandlerFunc(h.HandleNotify),
	}
	forceNotifyHandler := handlers.MethodHandler{
		"POST": http.HandlerFunc(h.HandleForceNotify),
	}
	getConceptHandler := handlers.MethodHandler{
		"GET": http.HandlerFunc(h.HandleGetConcept),
	}
	getConceptsHandler := handlers.MethodHandler{
		"GET": http.HandlerFunc(h.HandleGetConcepts),
	}

	router.Handle("/notify", notifyHandler)
	router.Handle("/force-notify", forceNotifyHandler)
	router.Handle("/concept/{uuid}", getConceptHandler)
	router.Handle("/concepts", getConceptsHandler)
}

type notificationRequest struct {
	notifySince   time.Time
	transactionID string
}

type ticker struct {
	ticker *time.Ticker
}

func (t *ticker) Tick() {
	<-t.ticker.C
}

func (t *ticker) Stop() {
	t.ticker.Stop()
}

func (h *Handler) processNotifyRequests() {
	for {
		h.ticker.Tick()

		if len(h.requestCh) == 0 {
			continue
		}

		n := notificationRequest{notifySince: maxTimeValue}
		for req := range h.requestCh {
			if n.notifySince.After(req.notifySince) {
				n = req
			}

			if len(h.requestCh) == 0 {
				break
			}
		}

		err := h.notifier.Notify(n.notifySince, n.transactionID)
		if err != nil {
			log.WithError(err).Errorf("Failed to notify for a change with transaction id %s since %v", n.transactionID, n.notifySince)
		}
	}
}

type responseData struct {
	Msg string
	Err error
}

func writeResponseData(w http.ResponseWriter, statusCode int, contentType string, msg string) {
	if contentType == "" {
		contentType = "text/plain"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(statusCode)
	_, _ = w.Write([]byte(msg))
}

func writeJSONResponseMessage(w http.ResponseWriter, statusCode int, resp responseData) {
	msg := `{"message": "` + resp.Msg + `"}`
	if resp.Err != nil {
		msg = `{"message": "` + resp.Msg + `", "error": "` + resp.Err.Error() + `"}`
	}
	writeResponseData(w, statusCode, "application/json", msg)
}

func validateLastChangeDate(change string) (time.Time, error) {
	lastChange, err := time.Parse(TimeFormat, change)
	if err != nil {
		return time.Time{}, fmt.Errorf("Date is not in the format %s", TimeFormat)
	}
	log.WithField("time", lastChange).Debug("Parsing notification time")
	lastChange = lastChange.Add(-10 * time.Millisecond)
	log.WithField("time", lastChange).Debug("Subtracting notification time wobble")

	if time.Since(lastChange) > LastChangeLimit {
		return time.Time{}, fmt.Errorf("Last change date should be time point in the last %.0f hours", LastChangeLimit.Hours())
	}
	return lastChange, nil
}
