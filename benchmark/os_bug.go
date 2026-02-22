//go:build ignore

package main

import (
	"fmt"
	"os"
)

func readConfig() string {
	data, err := os.ReadFile("config.yaml")
	fmt.Println("Config loaded")
	return string(data)
}
