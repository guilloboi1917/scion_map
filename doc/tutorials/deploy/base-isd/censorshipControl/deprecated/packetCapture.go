package deprecated

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
)

var (
	captureCmd        *exec.Cmd
	captureInProgress bool
	pcapFilePath      = "/data/traffic.pcap"
)

func startCapture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if captureInProgress {
		http.Error(w, "Capture already running", http.StatusConflict)
		return
	}

	// Start tcpdump in background
	captureCmd = exec.Command("tcpdump", "-i", "eth0", "-w", pcapFilePath)
	if err := captureCmd.Start(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to start capture: %v", err), http.StatusInternalServerError)
		return
	}

	captureInProgress = true
	log.Printf("Started tcpdump (PID: %d)", captureCmd.Process.Pid)

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Capture started (PID: %d)", captureCmd.Process.Pid)
}

func stopCapture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !captureInProgress {
		http.Error(w, "No capture in progress", http.StatusConflict)
		return
	}

	// Gracefully stop tcpdump
	if err := captureCmd.Process.Signal(os.Interrupt); err != nil {
		http.Error(w, fmt.Sprintf("Failed to stop capture: %v", err), http.StatusInternalServerError)
		return
	}

	// Wait for cleanup
	_, err := captureCmd.Process.Wait()
	if err != nil {
		log.Printf("Cleanup error: %v", err)
	}

	// Read and return the pcap file
	pcapData, err := os.ReadFile(pcapFilePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read pcap: %v", err), http.StatusInternalServerError)
		return
	}

	// Set headers for file download
	w.Header().Set("Content-Disposition", "attachment; filename=traffic.pcap")
	w.Header().Set("Content-Type", "application/vnd.tcpdump.pcap")
	w.WriteHeader(http.StatusOK)
	w.Write(pcapData)

	log.Printf("Capture stopped")

	// Cleanup
	captureInProgress = false
	os.Remove(pcapFilePath) // Optional: remove after sending
}

func main() {
	http.HandleFunc("/startcapture", startCapture)
	http.HandleFunc("/stopcapture", stopCapture)
	log.Println("API running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
