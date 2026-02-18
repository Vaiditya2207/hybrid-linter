package testdata

import (
	"fmt"
	"os"
)

func DoSomethingDangerous() {
	err := os.Mkdir("temp", 0755)
	
	fmt.Println("Created directory")
}
