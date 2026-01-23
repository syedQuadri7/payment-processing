package worker

import (
	"log"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"payment-processing/pkg/domain"
	"payment-processing/workflow"
)

// StartWorker starts the Temporal worker without database dependencies
// Deprecated: Use StartWorkerWithDependencies for full functionality
func StartWorker(c client.Client) error {
	return StartWorkerWithDependencies(c, nil)
}

// StartWorkerWithDependencies starts the Temporal worker with repository dependencies
func StartWorkerWithDependencies(c client.Client, repos *domain.Repositories) error {
	w := worker.New(c, workflow.TaskQueueName, worker.Options{})

	// Register workflows
	w.RegisterWorkflow(workflow.ProcessPaymentWorkflow)
	w.RegisterWorkflow(workflow.PaymentIntentWorkflow)

	// Register legacy activities
	w.RegisterActivity(workflow.ValidatePayment)
	w.RegisterActivity(workflow.ExecutePayment)

	// Create and register activities with dependencies
	var activities *workflow.Activities
	if repos != nil {
		activities = workflow.NewActivitiesWithDependencies(
			repos.DeclineCodes,
			repos.PaymentAttempts,
			repos.Outbox,
			repos.AuditLog,
		)
	} else {
		activities = workflow.NewActivities()
	}

	// Register all activity methods
	w.RegisterActivity(activities.AuthorizePayment)
	w.RegisterActivity(activities.CapturePayment)
	w.RegisterActivity(activities.VoidPayment)
	w.RegisterActivity(activities.PersistPaymentState)
	w.RegisterActivity(activities.WriteOutboxEvent)
	w.RegisterActivity(activities.WriteAuditLog)
	w.RegisterActivity(activities.GetPaymentIntent)
	w.RegisterActivity(activities.CreatePaymentAttempt)
	w.RegisterActivity(activities.CompletePaymentAttempt)
	w.RegisterActivity(activities.RecordPaymentAttempt)
	w.RegisterActivity(activities.ClassifyDecline)

	log.Println("Starting Temporal worker...")
	return w.Run(worker.InterruptCh())
}
