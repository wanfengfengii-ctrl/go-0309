// Command server is the runnable entry point for the shotcrete closure service.
// It opens the embedded persistent store, recovers leases, device calls and
// cycle state from disk, wires the HTTP API and serves the documented endpoints.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/app"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/httpapi"
)

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	dataPath := os.Getenv("DATA_PATH")
	if dataPath == "" {
		dataPath = "shotcrete.db"
	}

	svc, err := app.NewService(dataPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer func() {
		if err := svc.Close(); err != nil {
			log.Printf("close store: %v", err)
		}
	}()

	srv := httpapi.NewServer(svc)
	log.Printf("shotcrete closure service listening on %s (data=%s)", addr, dataPath)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
