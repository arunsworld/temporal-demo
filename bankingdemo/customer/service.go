package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
)

type service struct {
	ctx            context.Context
	workflowClient client.Client
	tmpl           *template.Template
}

func newService(ctx context.Context, c client.Client) *service {
	tmpl := template.Must(template.New("index").Parse(indexHTML))
	tmpl = template.Must(tmpl.New("success").Parse(successHTML))
	tmpl = template.Must(tmpl.New("status").Parse(statusHTML))
	tmpl = template.Must(tmpl.New("running").Parse(runningWorkflowHTML))
	result := &service{
		ctx:            ctx,
		workflowClient: c,
		tmpl:           tmpl,
	}
	result.registerHandlers()
	return result
}

func (s *service) registerHandlers() {
	http.HandleFunc("/", s.indexHandler)
	http.HandleFunc("/status", s.statusHandler)
}

type FormField struct {
	Field string
	Label string
}

var formFields = []FormField{
	{Field: "faccount", Label: "Source account"},
	{Field: "daccount", Label: "Destination account"},
	{Field: "amount", Label: "Amount"},
	{Field: "ref", Label: "Reference"},
}

func (s *service) indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		data := struct {
			FormFields []FormField
		}{
			FormFields: formFields,
		}
		if err := s.tmpl.ExecuteTemplate(w, "index", data); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "ERROR: %v", err)
			return
		}
		w.Header().Add("Content-Type", "text/html; charset=utf-8")
	} else {
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "ERROR: %v", err)
			return
		}
		req, err := parseFormFields(r)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "ERROR: %v", err)
			return
		}
		options := client.StartWorkflowOptions{
			TaskQueue: "clearing-house",
			ID:        fmt.Sprintf("MT: %s", req.Ref),
		}
		workflowRun, err := s.workflowClient.ExecuteWorkflow(s.ctx, options, "MoneyTransfer", req)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "ERROR: %v", err)
			return
		}
		data := struct {
			Ref   string
			RunID string
		}{
			Ref:   req.Ref,
			RunID: workflowRun.GetRunID(),
		}
		if err := s.tmpl.ExecuteTemplate(w, "success", data); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "ERROR: %v", err)
			return
		}
		w.Header().Add("Content-Type", "text/html; charset=utf-8")
	}
}

func parseFormFields(r *http.Request) (Request, error) {
	amtStr := r.Form.Get("amount")
	amt, err := strconv.ParseFloat(amtStr, 64)
	if err != nil {
		return Request{}, fmt.Errorf("amount is an invalid number")
	}
	req := Request{
		SourceBank:      "abbank",
		SourceAcc:       r.Form.Get("faccount"),
		DestinationBank: "bcbank",
		DestinationAcc:  r.Form.Get("daccount"),
		Ref:             r.Form.Get("ref"),
		Amount:          amt,
	}
	if req.SourceAcc == "" {
		return Request{}, fmt.Errorf("source account is empty")
	}
	if req.DestinationAcc == "" {
		return Request{}, fmt.Errorf("destination account is empty")
	}
	if req.Ref == "" {
		return Request{}, fmt.Errorf("reference is empty")
	}
	return req, nil
}

func (s *service) statusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		if err := s.tmpl.ExecuteTemplate(w, "status", nil); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "ERROR: %v", err)
			return
		}
		w.Header().Add("Content-Type", "text/html; charset=utf-8")
	} else {
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "ERROR: %v", err)
			return
		}
		ref := r.Form.Get("ref")
		if ref == "" {
			http.Redirect(w, r, "/status", http.StatusSeeOther)
			return
		}
		wid := fmt.Sprintf("MT: %s", ref)
		resp, err := s.workflowClient.DescribeWorkflowExecution(s.ctx, wid, "")
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Unable to get reference (%v). Go back and try again.", err)
			return
		}
		status := resp.WorkflowExecutionInfo.Status
		switch status {
		case enums.WORKFLOW_EXECUTION_STATUS_COMPLETED:
			runID := resp.WorkflowExecutionInfo.Execution.RunId
			resp := Response{}
			if err := s.workflowClient.GetWorkflow(s.ctx, wid, runID).Get(s.ctx, &resp); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "ERROR: %v", err)
				return
			}
			fmt.Fprint(w, resp.String())
		case enums.WORKFLOW_EXECUTION_STATUS_RUNNING:
			pad := []pendingActivityDetails{}
			for _, v := range resp.PendingActivities {
				lastFailure := "None"
				if v.LastFailure != nil {
					lastFailure = v.LastFailure.String()
				}
				pad = append(pad, pendingActivityDetails{
					ActivityName: v.ActivityType.Name,
					State:        v.State.String(),
					LastStarted:  time.Since(*v.LastStartedTime),
					LastFailure:  lastFailure,
				})
			}
			data := struct {
				PendingActivities []pendingActivityDetails
			}{
				PendingActivities: pad,
			}
			if err := s.tmpl.ExecuteTemplate(w, "running", data); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "ERROR: %v", err)
				return
			}
			w.Header().Add("Content-Type", "text/html; charset=utf-8")
		default:
			fmt.Fprintf(w, "ERROR: Workflow status: %s", status.String())
		}
	}
}

type pendingActivityDetails struct {
	ActivityName string
	State        string
	LastStarted  time.Duration
	LastFailure  string
}

const indexHTML = `
<!doctype html>
<html>
	<head>
		<meta charset="UTF-8">
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<title>Money Transfer App</title>
	</head>
	<body>
		<h1>Money Transfer App</h1>
		<form action="/" method="post">
			{{range $i, $v := .FormFields}}
			<div style="margin-bottom: 0.5rem;">
				<label for="{{.Field}}">{{.Label}}:</label>
				<input type="text" name="{{.Field}}" {{if eq $i 0}}autofocus{{end}}>
			</div>
			{{end}}
			
			<button type="submit">Go</button>
		</form>
	</body>
</html>
`

const successHTML = `
<!doctype html>
<html>
	<head>
		<meta charset="UTF-8">
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<title>Money Transfer App</title>
	</head>
	<body>
		<h1>Money Transfer App</h1>
		<h3>
			Request successful
		</h3>
		<div>
			<p>You can check status by visiting <a href="/status">this link.</a></p>
			<p>Use your reference number: {{.Ref}}</p>
			<p>Debug detail: Run ID: {{.RunID}}</p>
		</div>
		<div>
			<p><a href="/">Submit another request</a></p>
		</div>
	</body>
</html>
`

const statusHTML = `
<html>
	<head>
		<meta charset="UTF-8">
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<title>Money Transfer App - Status</title>
	</head>
	<body>
		<h1>Check Status</h1>
		<form action="/status" method="post">
			<div style="margin-bottom: 0.5rem;">
				<label for="ref">Reference:</label>
				<input type="text" name="ref" autofocus>
			</div>
			
			<button type="submit">Go</button>
		</form>
		<div>
			<p><a href="/">Submit a new request</a></p>
		</div>
	</body>
</html>
`

const runningWorkflowHTML = `
<html>
	<head>
		<meta charset="UTF-8">
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<title>Money Transfer App - In Progress</title>
	</head>
	<body>
		<h1>Money Transfer In Progress</h1>
		<h3>Pending Activities</h3>
		<ol>
			{{range .PendingActivities}}
			<li>{{.ActivityName}}. State: {{.State}}. Last Started: {{.LastStarted}}. Last failure: {{.LastFailure}}</li>
			{{end}}
		</ol>
	</body>
</html>
`
