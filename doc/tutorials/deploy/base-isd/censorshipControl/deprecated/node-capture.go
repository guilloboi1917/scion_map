package deprecated

import (
	"flag"
	"log"
	"os/exec"
)

func main() {
	start := flag.Bool("start", false, "Start capture")
	stop := flag.Bool("stop", false, "Stop capture")
	flag.Parse()

	switch {
	case *start:
		exec.Command("tcpdump", "-i", "eth0", "-w", "/data/capture.pcap").Start()
	case *stop:
		exec.Command("pkill", "tcpdump").Run()
	default:
		log.Println("Usage: node-capture -start|-stop")
	}
}
