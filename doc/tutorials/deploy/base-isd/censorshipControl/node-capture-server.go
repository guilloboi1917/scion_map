package main

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
)

func main() {
	http.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		out, err := exec.Command("systemctl", "start", "node-capture").CombinedOutput()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Error: %v\nOutput: %s", err, out)
			return
		}
		w.Write([]byte("Capture started via systemd"))
	})

	http.HandleFunc("/stop", func(w http.ResponseWriter, r *http.Request) {
		out, err := exec.Command("systemctl", "stop", "node-capture").CombinedOutput()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Error: %v\nOutput: %s", err, out)
			return
		}
		w.Write([]byte("Capture stopped via systemd"))
	})

	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		out, _ := exec.Command("systemctl", "is-active", "node-capture").CombinedOutput()
		w.Write(out)
	})

	log.Println("Management API running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}