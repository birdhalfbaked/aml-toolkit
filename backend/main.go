package main

import (
	"log"
	"net/http"

	"com.birdhalfbaked.aml-toolkit/internal/httpserver"
)

func main() {
	stack, err := httpserver.OpenStack(httpserver.Config{})
	if err != nil {
		log.Fatal(err)
	}
	defer stack.DB.Close()

	h := httpserver.NewHandler(stack, "", nil)
	addr := httpserver.ListenAddr()
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, h))
}
