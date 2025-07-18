package main

import (
	"flag"
	"log"
	"marchat/server"
	"net/http"
)

var adminKey = flag.String("admin-key", "", "Admin key for privileged commands like /clear")

func main() {
	flag.Parse()
	db := server.InitDB("chat.db")
	server.CreateSchema(db)

	hub := server.NewHub()
	go hub.Run()

	http.HandleFunc("/ws", server.ServeWs(hub, db))
	http.HandleFunc("/clear", server.ClearHandler(db, hub, *adminKey))

	log.Println("marchat WebSocket server running on :9090")
	log.Fatal(http.ListenAndServe(":9090", nil))
}
