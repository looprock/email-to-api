package main

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	hash, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(hash))
}
