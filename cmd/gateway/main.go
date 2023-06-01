package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"github.com/plgd-dev/go-coap/v3/message"
	"github.com/plgd-dev/go-coap/v3/message/codes"
	"github.com/plgd-dev/go-coap/v3/udp/server"
	"log"
	"net"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jlevesy/rgh/pkg/coap"
	"github.com/jlevesy/rgh/pkg/memberring"
	"github.com/plgd-dev/go-coap/v3/mux"
	coapNet "github.com/plgd-dev/go-coap/v3/net"
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

	coapListener, err := coapNet.NewListenUDP("udp", fmt.Sprintf(":%d", coapPort))
	if err != nil {
		log.Fatal("Can't listen coap", err)
	}
	coapSrv := udp.NewServer(
		options.WithContext(ctx),
		//options.WithMux(redirect(ring, coapSrv))
	)

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

func redirect(ring memberring.Ring, coapSrv *server.Server) mux.Handler {
	return mux.HandlerFunc(func(w mux.ResponseWriter, r *mux.Message) {
		rawQueries, err := r.Message.Queries()
		if err != nil {
			log.Println("could retrieve queries", rawQueries)
			coap.Abort(w)
			return
		}

		qs := parseQueries(rawQueries)

		key, ok := qs["key"]
		if !ok {
			log.Println("could retrieve key")
			coap.Abort(w)
			return
		}

		srv, err := ring.GetNode([]byte(key))
		if err != nil {
			log.Println("could not retrieve the node", err)
			coap.Abort(w)
			return
		}

		fmt.Println("Routing query to node", srv.Name)

		peer, err := net.ResolveUDPAddr("udp", srv.Addr.String()+":10000")
		if err != nil {
			log.Println("could resolve server addr", err)
			coap.Abort(w)
			return
		}

		conn, err := coapSrv.NewConn(peer)
		resp, err := conn.Do(r.Message)
		if err != nil {
			w.SetResponse(
				codes.InternalServerError,
				message.TextPlain,
				bytes.NewReader([]byte("something went wrong")),
			)
			return
		}

		w.SetMessage(resp)
	})
}

func parseQueries(queries []string) map[string]string {
	parsed := make(map[string]string, len(queries))

	for _, v := range queries {
		sp := strings.SplitN(v, "=", 2)
		if len(sp) != 2 {
			parsed[v] = ""
			continue
		}

		parsed[sp[0]] = sp[1]
	}

	return parsed
}

func logRequests(next mux.Handler) mux.Handler {
	return mux.HandlerFunc(func(w mux.ResponseWriter, r *mux.Message) {
		log.Printf("Received a  message %v\n", r.String())
		next.ServeCOAP(w, r)
	})
}
