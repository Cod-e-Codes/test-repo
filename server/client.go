package server

import (
	"database/sql"
	"log"
	"marchat/shared"
	"time"

	"github.com/gorilla/websocket"
)

const (
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10 // send pings at 90% of pongWait
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
	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
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
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				// Channel closed, send close message
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			switch v := msg.(type) {
			case shared.Message:
				err := c.conn.WriteJSON(v)
				if err != nil {
					log.Println("writePump error:", err)
					return
				}
			case WSMessage:
				err := c.conn.WriteJSON(v)
				if err != nil {
					log.Println("writePump error:", err)
					return
				}
			default:
				log.Println("writePump: unknown message type")
			}
		case <-ticker.C:
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Println("writePump ping error:", err)
				return
			}
		}
	}
}
