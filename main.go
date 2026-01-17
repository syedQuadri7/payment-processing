package main

import (
	"log"
	"net/http"
	"os"

	"go.temporal.io/sdk/client"

	"payment-processing/server"
	"payment-processing/worker"
)

func main() {
	temporalHost := os.Getenv("TEMPORAL_HOST")
	if temporalHost == "" {
		temporalHost = "localhost:7233"
	}

	c, err := client.Dial(client.Options{
		HostPort: temporalHost,
	})
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer c.Close()

	// Start worker in a goroutine
	go func() {
		if err := worker.StartWorker(c); err != nil {
			log.Fatalf("Worker failed: %v", err)
		}
	}()

	// Start HTTP server
	srv := server.New(c)

	http.HandleFunc("/payment", srv.HandlePayment)
	http.HandleFunc("/payment/status", srv.HandlePaymentStatus)
	http.HandleFunc("/health", srv.HandleHealth)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting HTTP server on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}
}
