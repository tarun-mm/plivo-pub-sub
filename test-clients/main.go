package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Message structures matching the server
type ClientMessage struct {
	Type      string       `json:"type"`
	Topic     string       `json:"topic,omitempty"`
	ClientID  string       `json:"client_id,omitempty"`
	Message   *MessageData `json:"message,omitempty"`
	LastN     int          `json:"last_n,omitempty"`
	RequestID string       `json:"request_id,omitempty"`
}

type MessageData struct {
	ID      string      `json:"id"`
	Payload interface{} `json:"payload"`
}

type ServerMessage struct {
	Type      string       `json:"type"`
	RequestID string       `json:"request_id,omitempty"`
	Topic     string       `json:"topic,omitempty"`
	Status    string       `json:"status,omitempty"`
	Message   *MessageData `json:"message,omitempty"`
	Error     *ErrorInfo   `json:"error,omitempty"`
	Timestamp string       `json:"ts"`
}

type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Color codes
const (
	ColorReset   = "\033[0m"
	ColorRed     = "\033[31m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorBlue    = "\033[34m"
	ColorMagenta = "\033[35m"
	ColorCyan    = "\033[36m"
	ColorBold    = "\033[1m"
	ColorDim     = "\033[2m"
)

type TestClient struct {
	conn     *websocket.Conn
	clientID string
	done     chan struct{}
}

func main() {
	serverURL := flag.String("url", "ws://localhost:8080", "WebSocket server URL")
	clientID := flag.String("client", "", "Client ID (auto-generated if not provided)")
	flag.Parse()

	// Generate client ID if not provided
	if *clientID == "" {
		*clientID = fmt.Sprintf("go-client-%d", rand.Intn(100000))
	}

	// Connect to WebSocket server
	wsURL := fmt.Sprintf("%s/ws?client_id=%s", *serverURL, *clientID)
	fmt.Printf("%s%sConnecting to: %s%s\n", ColorCyan, ColorBold, wsURL, ColorReset)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		log.Fatalf("%sConnection error: %v%s\n", ColorRed, err, ColorReset)
	}
	defer conn.Close()

	fmt.Printf("%s✓ Connected to server%s\n", ColorGreen, ColorReset)
	fmt.Printf("%sClient ID: %s%s\n\n", ColorCyan, *clientID, ColorReset)

	client := &TestClient{
		conn:     conn,
		clientID: *clientID,
		done:     make(chan struct{}),
	}

	// Start message reader goroutine
	go client.readMessages()

	// Handle interrupt signal
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	// Print help
	printHelp()

	// Read commands from stdin
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print(ColorGreen + "> " + ColorReset)

	go func() {
		for scanner.Scan() {
			input := strings.TrimSpace(scanner.Text())
			if input != "" {
				client.handleCommand(input)
			}
			fmt.Print(ColorGreen + "> " + ColorReset)
		}
		close(client.done)
	}()

	// Wait for interrupt or done signal
	select {
	case <-interrupt:
		fmt.Printf("\n%sReceived interrupt signal, closing connection...%s\n", ColorYellow, ColorReset)
	case <-client.done:
		fmt.Printf("\n%sExiting...%s\n", ColorYellow, ColorReset)
	}

	// Clean close
	err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		log.Printf("Close error: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
}

func (c *TestClient) readMessages() {
	// Set up ping/pong handlers to keep connection alive
	pongWait := 60 * time.Second
	c.conn.SetReadDeadline(time.Now().Add(pongWait))

	// Server sends pings, we respond with pongs automatically
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Also handle ping frames (respond with pong)
	c.conn.SetPingHandler(func(appData string) error {
		err := c.conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(10*time.Second))
		if err != nil {
			log.Printf("%sError sending pong: %v%s\n", ColorRed, err, ColorReset)
		}
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("%sConnection error: %v%s\n", ColorRed, err, ColorReset)
			}
			close(c.done)
			return
		}

		var serverMsg ServerMessage
		if err := json.Unmarshal(message, &serverMsg); err != nil {
			log.Printf("%sFailed to parse message: %v%s\n", ColorRed, err, ColorReset)
			continue
		}

		c.printServerMessage(serverMsg)
	}
}

func (c *TestClient) handleCommand(input string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}

	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "sub", "subscribe":
		c.handleSubscribe(parts)
	case "unsub", "unsubscribe":
		c.handleUnsubscribe(parts)
	case "pub", "publish":
		c.handlePublish(parts)
	case "ping":
		c.handlePing()
	case "help", "?":
		printHelp()
	case "quit", "exit":
		close(c.done)
	default:
		fmt.Printf("%sUnknown command: %s%s\n", ColorRed, cmd, ColorReset)
		fmt.Printf("%sType 'help' for available commands%s\n", ColorDim, ColorReset)
	}
}

func (c *TestClient) handleSubscribe(parts []string) {
	if len(parts) < 2 {
		fmt.Printf("%sUsage: sub <topic> [last_n]%s\n", ColorRed, ColorReset)
		return
	}

	topic := parts[1]
	lastN := 0
	if len(parts) > 2 {
		fmt.Sscanf(parts[2], "%d", &lastN)
	}

	msg := ClientMessage{
		Type:      "subscribe",
		Topic:     topic,
		ClientID:  c.clientID,
		LastN:     lastN,
		RequestID: uuid.New().String(),
	}

	c.sendMessage(msg)
}

func (c *TestClient) handleUnsubscribe(parts []string) {
	if len(parts) < 2 {
		fmt.Printf("%sUsage: unsub <topic>%s\n", ColorRed, ColorReset)
		return
	}

	topic := parts[1]
	msg := ClientMessage{
		Type:      "unsubscribe",
		Topic:     topic,
		ClientID:  c.clientID,
		RequestID: uuid.New().String(),
	}

	c.sendMessage(msg)
}

func (c *TestClient) handlePublish(parts []string) {
	if len(parts) < 3 {
		fmt.Printf("%sUsage: pub <topic> <data>%s\n", ColorRed, ColorReset)
		return
	}

	topic := parts[1]
	payload := strings.Join(parts[2:], " ")

	// Try to parse as JSON, otherwise use as string
	var parsedPayload interface{}
	if err := json.Unmarshal([]byte(payload), &parsedPayload); err != nil {
		parsedPayload = payload
	}

	msg := ClientMessage{
		Type:  "publish",
		Topic: topic,
		Message: &MessageData{
			ID:      uuid.New().String(),
			Payload: parsedPayload,
		},
		RequestID: uuid.New().String(),
	}

	c.sendMessage(msg)
}

func (c *TestClient) handlePing() {
	msg := ClientMessage{
		Type:      "ping",
		RequestID: uuid.New().String(),
	}
	c.sendMessage(msg)
}

func (c *TestClient) sendMessage(msg ClientMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("%sError marshaling message: %v%s\n", ColorRed, err, ColorReset)
		return
	}

	fmt.Printf("%s>> Sending: %s%s\n", ColorBlue, string(data), ColorReset)

	err = c.conn.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		log.Printf("%sError sending message: %v%s\n", ColorRed, err, ColorReset)
	}
}

func (c *TestClient) printServerMessage(msg ServerMessage) {
	timestamp := msg.Timestamp
	if t, err := time.Parse(time.RFC3339, msg.Timestamp); err == nil {
		timestamp = t.Local().Format("15:04:05")
	}

	switch msg.Type {
	case "ack":
		fmt.Printf("%s<< [%s] ACK:%s %s - %s\n", ColorGreen, timestamp, ColorReset, msg.Topic, msg.Status)

	case "event":
		fmt.Printf("%s<< [%s] EVENT:%s topic=%s\n", ColorCyan, timestamp, ColorReset, msg.Topic)
		fmt.Printf("   Message ID: %s\n", msg.Message.ID)
		payloadJSON, _ := json.MarshalIndent(msg.Message.Payload, "   ", "  ")
		fmt.Printf("   Payload: %s\n", string(payloadJSON))

	case "error":
		fmt.Printf("%s<< [%s] ERROR:%s %s\n", ColorRed, timestamp, ColorReset, msg.Error.Code)
		fmt.Printf("   %s\n", msg.Error.Message)

	case "pong":
		fmt.Printf("%s<< [%s] PONG%s\n", ColorMagenta, timestamp, ColorReset)

	case "info":
		fmt.Printf("%s<< [%s] INFO:%s %s\n", ColorYellow, timestamp, ColorReset, msg.Topic)

	default:
		data, _ := json.MarshalIndent(msg, "", "  ")
		fmt.Printf("%s<< [%s]%s %s\n", ColorDim, timestamp, ColorReset, string(data))
	}
}

func printHelp() {
	fmt.Printf("%s%sAvailable Commands:%s\n", ColorBold, ColorCyan, ColorReset)
	fmt.Printf("  %ssub <topic> [last_n]%s  - Subscribe to topic (optional: get last N messages)\n", ColorCyan, ColorReset)
	fmt.Printf("  %sunsub <topic>%s         - Unsubscribe from topic\n", ColorCyan, ColorReset)
	fmt.Printf("  %spub <topic> <data>%s    - Publish message to topic\n", ColorCyan, ColorReset)
	fmt.Printf("  %sping%s                  - Send ping to server\n", ColorCyan, ColorReset)
	fmt.Printf("  %shelp%s                  - Show this help\n", ColorCyan, ColorReset)
	fmt.Printf("  %squit%s                  - Exit client\n\n", ColorCyan, ColorReset)
	fmt.Printf("%sExample:%s\n", ColorDim, ColorReset)
	fmt.Printf("  sub orders 5\n")
	fmt.Printf("  pub orders {\"order_id\":\"123\",\"amount\":99.5}\n")
	fmt.Printf("  pub orders hello-world\n\n")
}
