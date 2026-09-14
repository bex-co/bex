// The "agent": answers 200 OK on every GET. MESSAGE is overridable so a redeploy
// can visibly change the response; PORT is injected by the platform.
//
// Each request is logged so the tenant-view canary (scripts/tenant-view-liveness.sh
// stage 2/4 type=app) always sees a fresh app-stream line after the wake request.
// Startup-only logging left that stage flaky once the pod stayed warm past the
// probe's 10-minute query window.
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	msg := os.Getenv("MESSAGE")
	if msg == "" {
		msg = "OK"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		fmt.Fprint(w, msg)
	})
	log.Printf("hello-go listening on %s: %q", port, msg)
	_ = http.ListenAndServe(":"+port, nil)
}
