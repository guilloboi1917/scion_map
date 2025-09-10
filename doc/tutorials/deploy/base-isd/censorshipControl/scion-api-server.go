// SCION AS Container API Design
// BASE_URL: http://<container_ip>:8080/api

// Packet Capture Endpoints
// POST   /capture/start      	# {interface: "eth0"}
// POST   /capture/stop       	# {capture_id: "123"}
// GET    /capture/status     	# Returns status of capture
// GET    /capture/files      	# Returns available pcap files
// GET    /capture/file/{id}  	# Download specific pcap

// Configuration Endpoints
// POST   /config/scion       	# {file: "topology.json", content: "{...}"}
// GET    /config/scion/{file}	# Read config file
// POST   /config/firewall    	# {rules: ["allow scion", "drop other"]}
// POST		/config/scion/path-policy	# {type: propagation | core_registration | up_registration | down_registration | all, file: "updatedPathPolicy.yaml"}
// GET		/config/scion/path-policy	# {returns path-policy files}
// POST		/config/scion/topology	# {file: "updatedTopology.json"}
// GET		/config/scion/topology	# returns topology file

// Packet Dispatch Endpoints
// POST		/dispatch/ping/start			# {dst: "10.100.0.11", count: "5"}
// POST		/dispatch/scionping/start	# {dst: "17:ffaa:1:1", count: "5"}

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
)

// Global state for managing the capture process
var (
	captureCmd    *exec.Cmd
	captureMutex  sync.Mutex // Protect concurrent access to captureCmd and captureState
	pcapDirectory = "/data/captures"
)

// Global state for managing pinging process
var (
	pingCmd   *exec.Cmd
	pingMutex sync.Mutex
)

// Global state for managing pinging process
var (
	scionPingCmd   *exec.Cmd
	scionPingMutex sync.Mutex
)

// General state for commands
type CommandState struct {
	InProgress bool      `json:"in_progress"`
	PID        int       `json:"pid,omitempty"`
	StartTime  time.Time `json:"start_time,omitempty"`
	OutputFile string    `json:"output_file,omitempty"`
}

// currentCapture holds the state of the most recent capture
var currentCapture CommandState

// currentPing holds the state of the most recent pinging command
var currentPing CommandState

// currentPing holds the state of the most recent pinging command
var currentScionPing CommandState

// HTTPHandleFunc for starting a capture using tcpdump
func startCapture(w http.ResponseWriter, r *http.Request) {
	// Check if correct method
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed - use POST", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var request struct {
		Interface string `json:"interface"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		request.Interface = "eth0"
	}

	// Fallback if no interface provided
	if request.Interface == "" {
		request.Interface = "eth0"
	}

	captureMutex.Lock()
	defer captureMutex.Unlock()

	// Check if capture already running
	if currentCapture.InProgress {
		http.Error(w, "Capture already in progress", http.StatusConflict)
		return
	}

	// Build the tcpdump command
	outputFile := fmt.Sprintf("%s/capture_%d.pcap", pcapDirectory, time.Now().Unix())
	args := []string{"-i", request.Interface, "-w", outputFile} // Defaulting to eth0 interface

	captureCmd = exec.Command("tcpdump", args...)

	// Start capture
	if err := captureCmd.Start(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to start capture: %v", err), http.StatusInternalServerError)
		return
	}

	// Update the global state
	currentCapture = CommandState{
		InProgress: true,
		PID:        captureCmd.Process.Pid,
		StartTime:  time.Now(),
		OutputFile: outputFile,
	}

	log.Printf("Started tcpdump (PID: %d) on %s", currentCapture.PID, request.Interface)

	// CRITICAL: Launch a goroutine to wait for the process to finish.
	// This prevents it from becoming a zombie and cleans up the state.
	go func() {
		// Wait for the process to exit
		err := captureCmd.Wait()

		// Lock the mutex to update the state safely
		captureMutex.Lock()
		defer captureMutex.Unlock()

		if err != nil {
			log.Printf("tcpdump process (PID: %d) finished with error: %v", currentCapture.PID, err)
		} else {
			log.Printf("tcpdump process (PID: %d) finished successfully", currentCapture.PID)
		}
		// Mark the capture as stopped, regardless of success or failure
		currentCapture.InProgress = false
		// Clear PID and other details if desired
		currentCapture.PID = 0
	}()

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Capture started on %s (PID: %d)", request.Interface, currentCapture.PID)
}

// stopCapture handles requests to stop the running capture.
func stopCapture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed - use POST", http.StatusMethodNotAllowed)
		return
	}

	captureMutex.Lock()
	defer captureMutex.Unlock()

	if !currentCapture.InProgress || captureCmd == nil || captureCmd.Process == nil {
		http.Error(w, "No capture is currently running", http.StatusConflict)
		return
	}

	// Send SIGTERM to stop tcpdump gracefully (it will finalize the pcap file)
	if err := captureCmd.Process.Signal(os.Interrupt); err != nil { // SIGINT is the standard way to stop tcpdump
		http.Error(w, fmt.Sprintf("Failed to stop capture: %v", err), http.StatusInternalServerError)
		return
	}
	// The already-running Wait() goroutine will handle the cleanup and update currentCapture.InProgress

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Stop signal sent to capture process (PID: %d)", currentCapture.PID)
}

// getCaptureStatus returns the current status of the capture.
func getCaptureStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed - use GET", http.StatusMethodNotAllowed)
		return
	}

	captureMutex.Lock()
	defer captureMutex.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(currentCapture)
}

func startPing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed - use POST", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Dst   string `json:"dst"`
		Count *int   `json:"count,omitempty"`
	}

	// Requires data sent to be Content-Type: application/json
	// Example:
	// curl -X POST \
	// -H "Content-Type: application/json" \
	// -d '{"dst":"10.100.0.100", "count":5}' \
	// http://10.100.0.25:8080/api/dispatch/ping/start
	// TODO: Fix count being wrongly parsed
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid Body", http.StatusBadRequest)
		return
	}

	if ip := net.ParseIP(request.Dst); ip == nil {
		http.Error(w, "Invalid dst IP", http.StatusBadRequest)
		return
	}

	// Could also do checks on Count

	pingMutex.Lock()
	defer pingMutex.Unlock()

	// Check if a ping is already running
	if currentPing.InProgress {
		http.Error(w, "A ping is already in progress", http.StatusConflict)
		return
	}

	// Build the command e.g. ping -c 5 10.100.0.11 or just ping 10.100.0.11 for a continously running ping
	// TODO: Empty count results in empty return from server and weird systemctl status
	args := []string{request.Dst}

	// Check if count provided
	if request.Count != nil {
		// Convert to string
		countStr := fmt.Sprintf("%d", *request.Count)

		// Append to args
		args = append([]string{"-c", countStr}, args...)
	}

	// Start the command
	pingCmd = exec.Command("ping", args...)

	if err := pingCmd.Start(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to start ping for dst: %s and count: %d", request.Dst, *request.Count), http.StatusInternalServerError)
		return
	}

	currentPing = CommandState{
		InProgress: true,
		PID:        pingCmd.Process.Pid,
		StartTime:  time.Now(),
	}

	log.Printf("Started ping (PID: %d), dst: %s count: %d", currentPing.PID, request.Dst, *request.Count)

	// Finally run the wait command

	go func() {
		err := pingCmd.Wait()

		// Lock to update safely
		pingMutex.Lock()
		defer pingMutex.Unlock()

		if err != nil {
			log.Printf("ping process (PID: %d) finished with error: %v", currentPing.PID, err)
		} else {
			log.Printf("ping process (PID: %d) finished successfully", currentPing.PID)
		}

		currentPing.InProgress = false
		currentPing.PID = 0
	}()

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Pinging started (PID: %d)", currentPing.PID)
}

func stopPing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed - use POST", http.StatusMethodNotAllowed)
		return
	}

	pingMutex.Lock()
	defer pingMutex.Unlock()

	if !currentPing.InProgress || pingCmd == nil || pingCmd.Process == nil {
		http.Error(w, "No ping in progress", http.StatusConflict)
		return
	}

	// Stop process
	if err := pingCmd.Process.Signal(os.Interrupt); err != nil {
		http.Error(w, fmt.Sprintf("Failed to stop ping: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Stop signal sent to capture process (PID: %d)", currentPing.PID)
}

func startScionPing(w http.ResponseWriter, r *http.Request) {
	return
}

func modifyPathPolicyConfig(w http.ResponseWriter, r *http.Request) {
	return
}

func modifyTopologyConfig(w http.ResponseWriter, r *http.Request) {
	return
}

func restartScionServices(w http.ResponseWriter, r *http.Request) {
	return
}




func main() {
	// Initialize the capture state
	currentCapture = CommandState{InProgress: false}

	// Register API endpoints
	http.HandleFunc("/api/capture/start", startCapture)
	http.HandleFunc("/api/capture/stop", stopCapture)        // New endpoint
	http.HandleFunc("/api/capture/status", getCaptureStatus) // New endpoint

	http.HandleFunc("/api/dispatch/ping/start", startPing)
	http.HandleFunc("/api/dispatch/ping/stop", stopPing)
	http.HandleFunc("/api/dispatch/scionping/start", startScionPing)

	log.Println("SCION AS Container API running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
