package server

import (
	"database/sql"
	"log"
	"marchat/shared"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan shared.Message
	db   *sql.DB
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	for {
		var msg shared.Message
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			log.Println("readPump error:", err)
			break
		}
		msg.CreatedAt = time.Now()
		InsertMessage(c.db, msg)
		c.hub.broadcast <- msg
	}
}

func (c *Client) writePump() {
	defer c.conn.Close()
	for msg := range c.send {
		err := c.conn.WriteJSON(msg)
		if err != nil {
			log.Println("writePump error:", err)
			break
		}
	}
}
