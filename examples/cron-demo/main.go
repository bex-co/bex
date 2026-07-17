// cron-demo: the payload of a bex cron_job. The operator schedules this as a
// Kubernetes CronJob; each run logs a timestamped message and exits immediately.
// No HTTP port, no Deployment, no Ingress — just periodic work.
package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	msg := os.Getenv("MESSAGE")
	if msg == "" {
		msg = "tick"
	}
	fmt.Printf("[%s] cron-demo: %s\n", time.Now().UTC().Format(time.RFC3339), msg)
}
