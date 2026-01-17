package worker

import (
	"log"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"payment-processing/workflow"
)

func StartWorker(c client.Client) error {
	w := worker.New(c, workflow.TaskQueueName, worker.Options{})

	w.RegisterWorkflow(workflow.ProcessPaymentWorkflow)
	w.RegisterActivity(workflow.ValidatePayment)
	w.RegisterActivity(workflow.ExecutePayment)

	log.Println("Starting Temporal worker...")
	return w.Run(worker.InterruptCh())
}
