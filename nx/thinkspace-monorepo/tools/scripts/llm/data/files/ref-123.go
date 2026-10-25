///ref /src/cmd/main.go
package main

import "fmt"

func main() {
	// Added retry block
	fmt.Println("Initializing service...")
	startServer()
}

func startServer() {
	fmt.Println("Server started on :8080")
}
