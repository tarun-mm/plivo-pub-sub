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
	"sync"
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
	Msg       string       `json:"msg,omitempty"`
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
	conn       *websocket.Conn
	clientID   string
	done       chan struct{}
	closeOnce  sync.Once
	showPrompt chan bool
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
		conn:       conn,
		clientID:   *clientID,
		done:       make(chan struct{}),
		showPrompt: make(chan bool, 1),
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

	// Input handler goroutine
	go func() {
		fmt.Print(ColorGreen + "> " + ColorReset)
		for {
			select {
			case <-client.done:
				return
			case <-client.showPrompt:
				fmt.Print(ColorGreen + "> " + ColorReset)
			default:
				if scanner.Scan() {
					input := strings.TrimSpace(scanner.Text())
					if input != "" {
						client.handleCommand(input)
					}
					// Show prompt after handling command
					select {
					case <-client.done:
						return
					default:
						fmt.Print(ColorGreen + "> " + ColorReset)
					}
				} else {
					// Scanner error or EOF
					if err := scanner.Err(); err != nil {
						log.Printf("Scanner error: %v", err)
					}
					client.close()
					return
				}
			}
		}
	}()

	// Wait for interrupt or done signal
	select {
	case <-interrupt:
		fmt.Printf("\n%sReceived interrupt signal, closing connection...%s\n", ColorYellow, ColorReset)
		client.close()
	case <-client.done:
		fmt.Printf("\n%sExiting...%s\n", ColorYellow, ColorReset)
	}

	// Give time for graceful close
	time.Sleep(200 * time.Millisecond)
}

// close safely closes the connection and done channel
func (c *TestClient) close() {
	c.closeOnce.Do(func() {
		// Send close message
		err := c.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			log.Printf("Close message error: %v", err)
		}

		// Close the connection
		c.conn.Close()

		// Signal done
		close(c.done)
	})
}

func (c *TestClient) readMessages() {
	defer func() {
		// Close connection if readMessages exits
		c.close()
	}()

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
			// Connection might be closing, don't log if done
			select {
			case <-c.done:
				return nil
			default:
				log.Printf("%sError sending pong: %v%s\n", ColorRed, err, ColorReset)
			}
		}
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		select {
		case <-c.done:
			return
		default:
		}

		_, message, err := c.conn.ReadMessage()
		if err != nil {
			// Check if this is expected close
			select {
			case <-c.done:
				// Already closing, no need to log
				return
			default:
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					log.Printf("%sConnection error: %v%s\n", ColorRed, err, ColorReset)
				}
				return
			}
		}

		var serverMsg ServerMessage
		if err := json.Unmarshal(message, &serverMsg); err != nil {
			log.Printf("%sFailed to parse message: %v%s\n", ColorRed, err, ColorReset)
			continue
		}

		c.printServerMessage(serverMsg)

		// Show prompt after server message
		select {
		case c.showPrompt <- true:
		default:
		}
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
		c.close()
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
		info := msg.Msg
		if msg.Topic != "" {
			info = fmt.Sprintf("%s (topic: %s)", msg.Msg, msg.Topic)
		}
		fmt.Printf("%s<< [%s] INFO:%s %s\n", ColorYellow, timestamp, ColorReset, info)

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
