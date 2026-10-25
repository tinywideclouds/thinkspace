///ref /src/cmd/main.go
package main

import "fmt"

// Added retry block
func main() {
	// Added retry block
	fmt.Println("Initializing service...")
	startServer()
}

func startServer() {
	fmt.Println("Server started on :8080")
}

func retryStartServer(retries int) {
	for i := 0; i < retries; i++ {
		fmt.Printf("Attempt %d to start server...\n", i+1)
		startServer()
		// Simulate a condition to retry (for demonstration purposes)
		if i < retries-1 {
			fmt.Println("Server failed to start, retrying...")
		}
	}
}
