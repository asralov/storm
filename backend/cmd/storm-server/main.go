package main

import (
	"log"
	"net/http"

	"github.com/yourusername/storm/internal/api"
)

func main() {
	server := api.NewServer()
	log.Println("Storm API running at http://127.0.0.1:8080")
	err := http.ListenAndServe("127.0.0.1:8080", server)
	if err != nil {
		log.Fatal(err)
	}
}
