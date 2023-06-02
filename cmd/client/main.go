package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"log"
	stdnet "net"
	"os/signal"
	"syscall"
	"time"

	"github.com/plgd-dev/go-coap/v3/message"
	"github.com/plgd-dev/go-coap/v3/message/codes"
	"github.com/plgd-dev/go-coap/v3/mux"
	"github.com/plgd-dev/go-coap/v3/net"
	"github.com/plgd-dev/go-coap/v3/options"
	"github.com/plgd-dev/go-coap/v3/udp"
	"golang.org/x/sync/errgroup"
)

func main() {
	var (
		addr   string
		listen string
		path   string
		key    string
		period time.Duration
	)

	flag.StringVar(&addr, "addr", "localhost:10000", "server address")
	flag.StringVar(&listen, "listen", ":10010", "listen address")
	flag.StringVar(&path, "path", "/call", "path to call")
	flag.StringVar(&key, "key", "", "routing key")
	flag.DurationVar(&period, "period", 5*time.Second, "period")
	flag.Parse()

	addrUDP, err := stdnet.ResolveUDPAddr("udp", addr)
	if err != nil {
		log.Fatal("Can't parse addr", err)
	}

	coapListener, err := net.NewListenUDP("udp", listen)
	if err != nil {
		log.Fatal("Cant listen", err)
	}
	defer coapListener.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	r := mux.NewRouter()
	r.Handle("/bidirectional", handleCall(listen))

	coapSrv := udp.NewServer(options.WithContext(ctx), options.WithMux(r))

	wg, grpCtx := errgroup.WithContext(ctx)

	wg.Go(func() error {
		go func() {
			<-grpCtx.Done()
			fmt.Println("Stopping the server")

			coapSrv.Stop()
		}()

		fmt.Println("Starting the server")
		return coapSrv.Serve(coapListener)
	})

	wg.Go(func() error {
		calls := map[string]int{}

		fmt.Println("Waiting for the server to be ready")
		// LOL.
		time.Sleep(time.Second)

		fmt.Println("Creating a new connection")
		co, err := coapSrv.NewConn(addrUDP)
		if err != nil {
			return err
		}

		defer co.Close()

		for {
			select {
			case <-grpCtx.Done():
				fmt.Println(calls)
				return nil
			default:
			}

			req, err := co.NewGetRequest(grpCtx, path)
			if err != nil {
				log.Printf("Cannot get request: %v", err)
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

			log.Printf("Response with messageID %d, seqno %d", resp.MessageID(), resp.Sequence())
			log.Printf("Response %d from %s with key %q", resp.Code(), string(b), query)

			calls[string(b)] += 1

			time.Sleep(period)
		}
	})

	if err := wg.Wait(); err != nil {
		fmt.Println("Client failed", err)
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

func handleCall(listen string) mux.Handler {
	return mux.HandlerFunc(func(w mux.ResponseWriter, req *mux.Message) {
		payload, err := io.ReadAll(req.Body())
		if err != nil {
			log.Printf("cannot read body: %v", err)
			return
		}

		if string(payload) == fmt.Sprintf("HELLO 127.0.0.1%s client", listen) {
			fmt.Println("received a calllll!!!! -----> ", string(payload))

			if err := w.SetResponse(codes.Content, message.TextPlain,
				bytes.NewReader([]byte(fmt.Sprintf("Congrats from %s client!!! You made it sweaty!!!", listen)))); err != nil {
				log.Printf("cannot set response: %v", err)
			}
			return
		}

		if err := w.SetResponse(codes.BadRequest, message.TextPlain, bytes.NewReader([]byte("Wrong client dumbass!!!"))); err != nil {
			log.Printf("cannot set response: %v", err)
		}
	})
}
