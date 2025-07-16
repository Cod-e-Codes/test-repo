package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"marchat/server"
	"marchat/shared"
)

func main() {
	db := server.InitDB("chat.db")
	server.CreateSchema(db)

	http.HandleFunc("/send", func(w http.ResponseWriter, r *http.Request) {
		var msg shared.Message
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			http.Error(w, "invalid", http.StatusBadRequest)
			return
		}
		msg.CreatedAt = time.Now()
		server.InsertMessage(db, msg)
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/messages", func(w http.ResponseWriter, r *http.Request) {
		messages := server.GetRecentMessages(db)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(messages)
	})

	http.HandleFunc("/clear", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		err := server.ClearMessages(db)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Failed to clear messages"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Messages cleared"))
	})

	log.Println("marchat server running on :9090")
	log.Fatal(http.ListenAndServe(":9090", nil))
}
