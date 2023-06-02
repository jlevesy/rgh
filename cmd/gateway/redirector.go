package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/jlevesy/rgh/pkg/coap"
	"github.com/jlevesy/rgh/pkg/memberring"
	"github.com/plgd-dev/go-coap/v3/message"
	"github.com/plgd-dev/go-coap/v3/message/codes"
	"github.com/plgd-dev/go-coap/v3/message/pool"
	"github.com/plgd-dev/go-coap/v3/mux"
	"github.com/plgd-dev/go-coap/v3/udp/client"
	"log"
	"math/rand"
	"net"
	"sync"
)

type NewConnFunc func(addr *net.UDPAddr) (*client.Conn, error)

var (
	ErrClientNotKnown = errors.New("client not known")
)

type Redirector struct {
	newConnFunc NewConnFunc
	ring        memberring.Ring

	m       sync.Mutex
	servers map[string]mux.Conn
}

func (r *Redirector) SetNewConnFunc(newConnFunc NewConnFunc) {
	r.newConnFunc = newConnFunc
}

func (r *Redirector) Redirect(w mux.ResponseWriter, msg *mux.Message) {
	rawQueries, err := msg.Message.Queries()
	if err != nil {
		log.Println("could retrieve queries", rawQueries)
		coap.Abort(w)
		return
	}

	qs := parseQueries(rawQueries)

	key, ok := qs["key"]
	if ok {
		log.Printf("Client requqest with messageID %d, seqno %d", msg.MessageID(), msg.Sequence())

		srvConn, err := r.getServerConnection(key)
		if err != nil {
			coap.Abort(w)
			return
		}

		clientMsg := msg.Message
		resp, err := r.redirectToServer(srvConn, w.Conn(), clientMsg, rawQueries)
		if err != nil {
			w.SetResponse(
				codes.InternalServerError,
				message.TextPlain,
				bytes.NewReader([]byte(err.Error())),
			)
			return
		}

		// Then clone the response into the response writer, and voila.
		messageID := w.Message().MessageID()
		if err := resp.Clone(w.Message()); err != nil {
			coap.Abort(w)
			return
		}

		log.Printf("Client response with messageID %d, seqno %d", messageID, w.Message().Sequence())

		return
	}

	log.Println("unknown request")
	w.SetResponse(
		codes.NotImplemented,
		message.TextPlain,
		bytes.NewReader([]byte("unknown request")),
	)
}

func (r *Redirector) getServerConnection(key string) (mux.Conn, error) {
	r.m.Lock()
	defer r.m.Unlock()

	if conn, ok := r.servers[key]; ok {
		return conn, nil
	}

	node, err := r.ring.GetNode([]byte(key))
	if err != nil {
		log.Println("could not retrieve the node", err)
		return nil, err
	}

	peer, err := net.ResolveUDPAddr("udp", node.Addr.String()+":"+getRandomPort(3))
	if err != nil {
		log.Println("could resolve server addr", err)
		return nil, err
	}

	fmt.Println(fmt.Sprintf("Redirecting to %s", peer.String()))

	conn, err := r.newConnFunc(peer)
	if err != nil {
		log.Println("could resolve server addr", err)
		return nil, err
	}

	r.servers[key] = conn
	return conn, nil
}

func (r *Redirector) redirectToServer(srvConn mux.Conn, clientConn mux.Conn, clientMsg *pool.Message, qs []string) (*pool.Message, error) {
	srvMsg := srvConn.AcquireMessage(context.Background())
	if err := clientMsg.Clone(srvMsg); err != nil {
		return nil, err
	}

	srvMsg.Options().Remove(message.URIQuery)
	for _, q := range qs {
		srvMsg.Options().Add(message.Option{ID: message.URIQuery, Value: []byte(q)})
	}

	// adding identifier of client request
	srvMsg.AddQuery(fmt.Sprintf("originalAddr=%s", clientConn.RemoteAddr().String()))

	srvResp, err := srvConn.Do(srvMsg)
	if err != nil {
		return nil, err
	}

	return srvResp, nil
}

func (r *Redirector) redirectToClient(clientConn mux.Conn, serverMsg *pool.Message) (*pool.Message, error) {
	clientMsg := clientConn.AcquireMessage(context.Background())
	if err := serverMsg.Clone(clientMsg); err != nil {
		return nil, err
	}

	clientResp, err := clientConn.Do(clientMsg)
	if err != nil {
		return nil, err
	}

	return clientResp, nil
}

func getRandomPort(num int) string {
	min := 10001
	max := 10000 + num
	return fmt.Sprint(rand.Intn(max-min) + min)
}

func NewRedirector(ring memberring.Ring) *Redirector {
	return &Redirector{
		ring:    ring,
		servers: make(map[string]mux.Conn),
	}
}
