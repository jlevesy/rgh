package main

import (
	"fmt"
	"github.com/plgd-dev/go-coap/v3/mux"
	"github.com/plgd-dev/go-coap/v3/udp/client"
	"log"
	"net"
	"sync"
)

type NewConnFunc func(addr *net.UDPAddr) (*client.Conn, error)

type Connections struct {
	m sync.Mutex

	newConnFunc NewConnFunc
	clients     map[string]mux.Conn
}

func (c *Connections) registerClientConnection(addr string) error {
	knownConn := c.getClientConnection(addr)
	if knownConn != nil {
		return nil
	}

	log.Println(fmt.Sprintf("new client connection: %s", addr))

	clientAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}

	conn, err := c.newConnFunc(clientAddr)
	if err != nil {
		return err
	}

	c.m.Lock()
	defer c.m.Unlock()
	c.clients[addr] = conn
	return nil
}

func (c *Connections) getClientConnection(addr string) mux.Conn {
	c.m.Lock()
	defer c.m.Unlock()

	if client, ok := c.clients[addr]; ok {
		return client
	}

	return nil
}
