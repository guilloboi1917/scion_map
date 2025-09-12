// SCION AS Container API Design
// BASE_URL: http://<container_ip>:8080/api

// Packet Capture Endpoints
// POST   /capture/start      	# {interface: "eth0"} 				// curl -X POST -d {"interface":"eth0"} http://10.100.0.11:8080/api/capture/start --> Capture started on eth0 (PID: 108)
// POST   /capture/stop       	# {capture_id: "123"}				// curl -X POST http://10.100.0.11:8080/api/capture/stop --> Stop signal sent to capture process (PID: 108)
// GET    /capture/status     	# Returns status of capture 		// curl http://10.100.0.11:8080/api/capture/status --> {"in_progress":true,"pid":108,"start_time":"2025-09-12T14:06:25.163857877Z","output_file":"/var/lib/scion-api-server/packet-captures/capture_1757685985.pcap"}
// GET    /capture/files      	# Returns available pcap files		// curl http://10.100.0.11:8080/api/capture/files --> [{"index":0,"name":"capture_1757685985.pcap","size":24}]
// GET    /capture/file/{id}  	# Download specific pcap			// NOT YET IMPLEMENTED

// Configuration Endpoints
// POST   /config/scion       	# {file: "topology.json", content: "{...}"}
// GET    /config/scion/{file}	# Read config file
// POST   /config/firewall    	# {rules: ["allow scion", "drop other"]}
// POST		/config/scion/path-policy	# {type: propagation | core_registration | up_registration | down_registration | all, file: "updatedPathPolicy.yaml"}
// GET		/config/scion/path-policy	# {returns path-policy files}
// POST		/config/scion/topology	# {file: "updatedTopology.json"}
// GET		/config/scion/topology	# returns topology file

// Packet Dispatch Endpoints
// POST		/dispatch/ping/start			# {dst: "10.100.0.11", count: "5"}		// curl -X POST -d '{"dst":"10.100.0.21","count":5}' http://10.100.0.11:8080/api/dispatch/ping/start --> Pinging started (PID: 113)
// GET		/dispatch/ping/stop														// curl -X POST http://10.100.0.11:8080/api/dispatch/ping/stop --> No ping in progress | Stop signal sent to capture process (PID: 114)
// GET		/dispatch/ping/files													// curl http://10.100.0.11:8080/api/dispatch/ping/files --> [{"index":0,"name":"ping_1757686184.log","size":498},{"index":1,"name":"ping_1757686301.log","size":616},{"index":2,"name":"ping_1757686354.log","size":977}]
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
	captureCmd   *exec.Cmd
	captureMutex sync.Mutex // Protect concurrent access to captureCmd and captureState
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

// locations where to store data
var pingResultLocation = "/var/lib/scion-api-server/ping-results"
var scionPingResultLocation = "/var/lib/scion-api-server/scion-ping-results"
var packetCapturesResultLocation = "/var/lib/scion-api-server/packet-captures"

// Struct for file information
type FileInfo struct {
	Index int32  `json:"index"`
	Name  string `json:"name"`
	Size  int64  `json:"size"`
}

func getFilesFromDirectory(directory string) ([]FileInfo, error) {
	// Read all files from the specified directory
	files, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	fileInfos := make([]FileInfo, 0)

	for idx, file := range files {
		if !file.IsDir() {
			info, err := file.Info()
			if err != nil {
				// Skip files we can't get info for
				continue
			}

			fileInfos = append(fileInfos, FileInfo{
				Index: int32(idx),
				Name:  file.Name(),
				Size:  info.Size(),
			})
		}
	}

	return fileInfos, nil
}

//// HANDLERS ////

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
	outputFile := fmt.Sprintf("%s/capture_%d.pcap", packetCapturesResultLocation, time.Now().Unix())
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

// list all available capture files
func getAvailableCaptures(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed - use GET", http.StatusMethodNotAllowed)
		return
	}

	fileInfos, err := getFilesFromDirectory(packetCapturesResultLocation)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fileInfos)
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

	var countDisplay string = "continuous" // Default display value

	// Check if count provided
	if request.Count != nil {
		// Convert to string
		countStr := fmt.Sprintf("%d", *request.Count)

		// Append to args
		args = append([]string{"-c", countStr}, args...)

		// To display for logging
		countDisplay = fmt.Sprintf("%d", *request.Count)
	}

	// We want stdout of the ping command to a file
	outputFile := fmt.Sprintf("%s/ping_%d.log", pingResultLocation, time.Now().Unix())

	// Start the command
	pingCmd = exec.Command("ping", args...)

	// Create output file
	file, err := os.OpenFile(outputFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create output file: %v", err), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	pingCmd.Stdout = file
	pingCmd.Stderr = file

	if err := pingCmd.Start(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to start ping for dst: %s and count: %s", request.Dst, countDisplay), http.StatusInternalServerError)
		return
	}

	currentPing = CommandState{
		InProgress: true,
		PID:        pingCmd.Process.Pid,
		StartTime:  time.Now(),
		OutputFile: outputFile,
	}

	log.Printf("Started ping (PID: %d), dst: %s count: %s", currentPing.PID, request.Dst, countDisplay)

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

func getAvailablePingResults(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed - use GET", http.StatusMethodNotAllowed)
		return
	}

	fileInfos, err := getFilesFromDirectory(pingResultLocation)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fileInfos)

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
	http.HandleFunc("/api/capture/files", getAvailableCaptures)

	http.HandleFunc("/api/dispatch/ping/start", startPing)
	http.HandleFunc("/api/dispatch/ping/stop", stopPing)
	http.HandleFunc("/api/dispatch/scionping/start", startScionPing)
	http.HandleFunc("/api/dispatch/ping/files", getAvailablePingResults)

	log.Println("SCION AS Container API running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
