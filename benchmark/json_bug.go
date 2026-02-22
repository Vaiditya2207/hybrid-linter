//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
)

type User struct {
	Name string
}

func parseUser(data []byte) *User {
	var u User
	err := json.Unmarshal(data, &u)
	fmt.Println(u.Name)
	return &u
}
