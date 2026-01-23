package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.temporal.io/sdk/client"

	"payment-processing/workflow"
)

type Server struct {
	temporalClient client.Client
}

func New(c client.Client) *Server {
	return &Server{temporalClient: c}
}

type PaymentRequest struct {
	Amount float64 `json:"amount"`
	From   string  `json:"from"`
	To     string  `json:"to"`
}

type PaymentResponse struct {
	WorkflowID string `json:"workflow_id"`
	RunID      string `json:"run_id"`
	Status     string `json:"status"`
	Message    string `json:"message"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func (s *Server) HandlePayment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}

	var req PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid JSON payload"})
		return
	}

	paymentID := uuid.New().String()
	input := workflow.LegacyPaymentInput{
		ID:     paymentID,
		Amount: req.Amount,
		From:   req.From,
		To:     req.To,
	}

	workflowOptions := client.StartWorkflowOptions{
		ID:        "payment-" + paymentID,
		TaskQueue: workflow.TaskQueueName,
	}

	we, err := s.temporalClient.ExecuteWorkflow(context.Background(), workflowOptions, workflow.ProcessPaymentWorkflow, input)
	if err != nil {
		log.Printf("Failed to start workflow: %v", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to start payment workflow"})
		return
	}

	log.Printf("Started workflow: WorkflowID=%s, RunID=%s", we.GetID(), we.GetRunID())

	writeJSON(w, http.StatusAccepted, PaymentResponse{
		WorkflowID: we.GetID(),
		RunID:      we.GetRunID(),
		Status:     "accepted",
		Message:    "Payment workflow started",
	})
}

func (s *Server) HandlePaymentStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}

	workflowID := r.URL.Query().Get("workflow_id")
	if workflowID == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "workflow_id is required"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	desc, err := s.temporalClient.DescribeWorkflowExecution(ctx, workflowID, "")
	if err != nil {
		log.Printf("Failed to describe workflow: %v", err)
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "Workflow not found"})
		return
	}

	status := desc.WorkflowExecutionInfo.Status.String()
	writeJSON(w, http.StatusOK, map[string]string{
		"workflow_id": workflowID,
		"status":      status,
	})
}

func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
