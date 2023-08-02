package main

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"sync"

	"github.com/google/uuid"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
)

type Request struct {
	SourceBank, DestinationBank string
	SourceAcc, DestinationAcc   string
	Amount                      float64
	Ref                         string
}

type service struct {
	mu             sync.RWMutex
	allRequests    map[string]Request
	tokenMap       map[string][]byte
	workflowClient client.Client
}

func newService(c client.Client) *service {
	result := &service{
		allRequests:    make(map[string]Request),
		tokenMap:       make(map[string][]byte),
		workflowClient: c,
	}
	result.registerHandlers()
	return result
}

func (s *service) registerHandlers() {
	http.HandleFunc("/", s.listHandler)
	http.HandleFunc("/action", s.actionHandler)
}

func (s *service) listHandler(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.allRequests) == 0 {
		_, _ = fmt.Fprint(w, "<h1>MONEY LAUNDERING SERVICE</h1>"+
			"<h3>No pending approvals</h3>")
		return
	}
	_, _ = fmt.Fprint(w, "<h1>MONEY LAUNDERING SERVICE</h1>"+"<a href=\"/\">HOME</a>"+
		"<h3>All approval requests:</h3><table border=1><tr><th>From</th><th>To</th><th>Amount</th><th>Reference</th><th>Action</th>")
	var keys []string
	for k := range s.allRequests {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, id := range keys {
		req := s.allRequests[id]
		actionLink := fmt.Sprintf("<a href=\"/action?type=approve&id=%s\">"+
			"<button style=\"background-color:#4CAF50;\">APPROVE</button></a>"+
			"&nbsp;&nbsp;<a href=\"/action?type=reject&id=%s\">"+
			"<button style=\"background-color:#f44336;\">REJECT</button></a>", id, id)
		_, _ = fmt.Fprintf(w, "<tr><td>%s [%s]</td><td>%s [%s]</td><td>%.2f</td><td>%s</td><td>%s</td></tr>",
			req.SourceBank, req.SourceAcc, req.DestinationBank, req.DestinationAcc,
			req.Amount, req.Ref, actionLink)
	}
	_, _ = fmt.Fprint(w, "</table>")
}

func (s *service) actionHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	s.mu.RLock()
	_, ok := s.allRequests[id]
	s.mu.RUnlock()
	if !ok {
		_, _ = fmt.Fprint(w, "ERROR:INVALID_ID")
		return
	}
	actionType := r.URL.Query().Get("type")
	if !(actionType == "approve" || actionType == "reject") {
		_, _ = fmt.Fprint(w, "ERROR:INVALID_ACTION_TYPE")
		return
	}
	s.mu.RLock()
	token, ok := s.tokenMap[id]
	s.mu.RUnlock()
	if !ok {
		_, _ = fmt.Fprint(w, "ERROR:INVALID_ID_FOR_TOKEN")
		return
	}
	if err := s.workflowClient.CompleteActivity(context.Background(), token, actionType, nil); err != nil {
		_, _ = fmt.Fprint(w, "ERROR:"+err.Error())
		return
	}
	s.mu.Lock()
	delete(s.allRequests, id)
	s.mu.Unlock()

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *service) temporalActivity(ctx context.Context, req Request) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := uuid.NewString()
	token := activity.GetInfo(ctx).TaskToken

	s.allRequests[id] = req
	s.tokenMap[id] = token

	return "", activity.ErrResultPending
}
