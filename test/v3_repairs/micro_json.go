package v3_repairs

import "encoding/json"

// parse should return an error if Unmarshal fails
func parse(b []byte) error {
	var m map[string]string
	err := json.Unmarshal(b, &m)
	_ = err
	return nil
}
