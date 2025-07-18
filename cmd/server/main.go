package main

import (
	"log"
	"marchat/server"
	"net/http"
)

func main() {
	db := server.InitDB("chat.db")
	server.CreateSchema(db)

	hub := server.NewHub()
	go hub.Run()

	http.HandleFunc("/ws", server.ServeWs(hub, db))

	log.Println("marchat WebSocket server running on :9090")
	log.Fatal(http.ListenAndServe(":9090", nil))
}
