package deprecated

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
)

func main() {
	node := flag.String("node", "", "Node IP (10.100.0.x)")
	start := flag.Bool("start", false, "Start capture")
	stop := flag.Bool("stop", false, "Stop capture")
	flag.Parse()

	if *node == "" {
		fmt.Println("Must specify -node")
		os.Exit(1)
	}

	url := fmt.Sprintf("http://%s:8080", *node)
	var endpoint string
	switch {
	case *start:
		endpoint = "/start"
	case *stop:
		endpoint = "/stop"
	default:
		fmt.Println("Usage: monitor-cli -node <IP> -start|-stop")
		return
	}

	resp, err := http.Post(url+endpoint, "text/plain", nil)
	if err != nil {
		fmt.Printf("Error contacting %s: %v\n", *node, err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("%s: %s\n", *node, string(body))
}
