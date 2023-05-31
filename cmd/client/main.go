package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/plgd-dev/go-coap/v3/udp"
)

func main() {
	var (
		addr string
		path string
		key  string
	)

	flag.StringVar(&addr, "addr", "localhost:10000", "server address")
	flag.StringVar(&path, "path", "/call", "path to call")
	flag.StringVar(&key, "key", "", "routing key")
	flag.Parse()

	co, err := udp.Dial(addr)
	if err != nil {
		log.Fatalf("Error dialing: %v", err)
	}
	defer co.Close()

	calls := map[string]int{}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			fmt.Println(calls)
			return
		default:
		}

		req, err := co.NewGetRequest(ctx, path)
		if err != nil {
			log.Printf("Cannot get response: %v", err)
			continue
		}

		query := genQuery(key)

		req.AddQuery(query)

		resp, err := co.Do(req)
		if err != nil {
			log.Printf("Cannot get response: %v", err)
			continue
		}

		b, _ := io.ReadAll(resp.Body())

		log.Printf("Response %d from %s with key %q", resp.Code(), string(b), query)

		calls[string(b)] += 1

		time.Sleep(10 * time.Millisecond)
	}
}

func genQuery(key string) string {
	if key == "" {
		b := make([]byte, 10)
		_, _ = rand.Read(b)
		key = string(b)
	}

	return "key=" + key
}
