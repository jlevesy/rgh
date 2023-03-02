package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/hashicorp/memberlist"
)

func main() {
	var (
		memberName string
		joinAddr   string
		gossipPort int
	)

	flag.StringVar(&memberName, "name", "", "name of the member")
	flag.StringVar(&joinAddr, "join-address", "127.0.0.1", "join address")
	flag.IntVar(&gossipPort, "gossip-port", 7946, "port to use for the gossiping")

	flag.Parse()

	cfg := memberlist.DefaultLocalConfig()

	cfg.Name = memberName
	cfg.AdvertisePort = gossipPort
	cfg.BindPort = gossipPort

	list, err := memberlist.Create(cfg)
	if err != nil {
		log.Fatal("Failed to create memberlist: ", err)
	}

	// Join an existing cluster by specifying at least one known member.
	_, err = list.Join([]string{joinAddr})
	if err != nil {
		log.Fatal("Failed to join cluster:", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	dumpMemberlist(ctx, list)

	log.Println("Received a signal", ctx.Err())

	if err := list.Leave(time.Second); err != nil {
		log.Println("can't leave", err)
	}

	if err := list.Shutdown(); err != nil {
		log.Println("can't shutdown", err)
	}
}

func dumpMemberlist(ctx context.Context, list *memberlist.Memberlist) {
	for {
		select {
		case <-time.Tick(time.Second):
			fmt.Println(
				"NumMembers:",
				list.NumMembers(),
				"HealthScore:",
				list.GetHealthScore(),
			)

			for _, node := range list.Members() {
				fmt.Println(
					"Node:",
					node.Name,
					"Addr:",
					node.Address(),
					"Meta:",
					node.Meta,
				)
			}

		case <-ctx.Done():
			return
		}
	}
}
