package server

import (
	"database/sql"
	"log"
	"marchat/shared"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan interface{}
	db       *sql.DB
	username string
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
		switch v := msg.(type) {
		case shared.Message:
			err := c.conn.WriteJSON(v)
			if err != nil {
				log.Println("writePump error:", err)
				break
			}
		case WSMessage:
			err := c.conn.WriteJSON(v)
			if err != nil {
				log.Println("writePump error:", err)
				break
			}
		default:
			log.Println("writePump: unknown message type")
		}
	}
}
