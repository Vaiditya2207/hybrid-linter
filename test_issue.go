package main

import "fmt"

func maybeError() error {
	return nil
}

func main() {
	err := maybeError()
	fmt.Println("Result:", err)
	
	// Unhandled error below
	err = maybeError()
}
