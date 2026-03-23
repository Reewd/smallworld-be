package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/net/websocket"
)

func main() {
	var (
		rawURL  string
		token   string
		timeout time.Duration
	)

	flag.StringVar(&rawURL, "url", "http://localhost:8080/v1/ws", "websocket endpoint url")
	flag.StringVar(&token, "token", "", "bearer token")
	flag.DurationVar(&timeout, "timeout", 3*time.Second, "connection timeout")
	flag.Parse()

	if strings.TrimSpace(token) == "" {
		log.Fatal("token is required")
	}

	wsURL, origin, err := websocketURL(rawURL)
	if err != nil {
		log.Fatalf("invalid websocket url: %v", err)
	}

	config, err := websocket.NewConfig(wsURL, origin)
	if err != nil {
		log.Fatalf("new websocket config: %v", err)
	}
	config.Header = http.Header{
		"Authorization": []string{"Bearer " + token},
	}

	conn, err := websocket.DialConfig(config)
	if err != nil {
		log.Fatalf("websocket dial failed: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))

	fmt.Fprintln(os.Stdout, "websocket connected")
}

func websocketURL(raw string) (string, string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", err
	}

	origin := *u
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
		origin.Scheme = "http"
	case "https":
		u.Scheme = "wss"
		origin.Scheme = "https"
	case "ws", "wss":
	default:
		return "", "", fmt.Errorf("unsupported scheme %q", u.Scheme)
	}

	return u.String(), origin.String(), nil
}
