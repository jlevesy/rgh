package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/jlevesy/rgh/pkg/coap"
	"github.com/jlevesy/rgh/pkg/memberring"
	"github.com/plgd-dev/go-coap/v3/net"
	"github.com/plgd-dev/go-coap/v3/options"
	"github.com/plgd-dev/go-coap/v3/udp"
	"golang.org/x/sync/errgroup"
)

func main() {
	var (
		memberName string
		joinAddr   string
		gossipPort int
		coapPort   int
	)

	flag.StringVar(&memberName, "name", "", "name of the member")
	flag.StringVar(&joinAddr, "join-address", "127.0.0.1", "join address")
	flag.IntVar(&gossipPort, "gossip-port", 7946, "port to use for the gossiping")
	flag.IntVar(&coapPort, "coap-port", 10000, "port to use for coap")

	flag.Parse()

	cfg := memberring.NewDefaultLocalConfig(memberring.RoleWatcher)
	cfg.List.Name = memberName
	cfg.List.AdvertisePort = gossipPort
	cfg.List.BindPort = gossipPort

	ring, err := memberring.New(cfg, []string{joinAddr})
	if err != nil {
		log.Fatal("Failed to create the ring: ", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	coapListener, err := net.NewListenUDP("udp", fmt.Sprintf(":%d", coapPort))
	if err != nil {
		log.Fatal("Can't listen coap", err)
	}

	forwarder := coap.NewForwarder(ring)

	coapSrv := udp.NewServer(
		options.WithContext(ctx),
		options.WithMux(forwarder),
	)

	forwarder.SetServer(coapSrv)

	var wg errgroup.Group

	log.Println("server is running, press ctrl+C to exit")

	wg.Go(func() error { return coapSrv.Serve(coapListener) })

	<-ctx.Done()

	log.Println("Received a signal", ctx.Err())

	if err := ring.List().Leave(time.Second); err != nil {
		log.Println("can't leave", err)
	}

	if err := ring.List().Shutdown(); err != nil {
		log.Println("can't shutdown", err)
	}

	coapSrv.Stop()

	if err := wg.Wait(); err != nil {
		log.Println("Server reported an error", err)
	}

	log.Println("Exited")
}
