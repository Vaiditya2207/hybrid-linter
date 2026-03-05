package main

import (
	"os"
)

// mini should return an error if ReadFile fails
func mini() error {
	_, err := os.ReadFile("nonexistent.txt")
	_ = err
	return nil
}
