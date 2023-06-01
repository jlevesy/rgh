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
	"github.com/plgd-dev/go-coap/v3/udp/server"
	"log"
	"net"
)

var (
	ErrClientNotKnown = errors.New("client not known")
)

type Redirector struct {
	coapSrv *server.Server
	ring    memberring.MemberRing

	clients map[string]mux.Conn
	servers map[string]mux.Conn
}

func (r *Redirector) Redirect(w mux.ResponseWriter, msg *mux.Message) {
	rawQueries, err := msg.Message.Queries()
	if err != nil {
		log.Println("could retrieve queries", rawQueries)
		coap.Abort(w)
		return
	}

	addr := w.Conn().RemoteAddr()
	qs := parseQueries(rawQueries)

	key, ok := qs["key"]
	if ok {
		srvConn, err := r.getServerConnection(key)
		if err != nil {
			coap.Abort(w)
			return
		}

		clientMsg := msg.Message
		clientConn := r.getOrRegisterClientConnection(addr.String(), w)
		resp, err := r.redirectToServer(srvConn, clientConn, clientMsg)
		if err != nil {
			w.SetResponse(
				codes.InternalServerError,
				message.TextPlain,
				bytes.NewReader([]byte(err.Error())),
			)
			return
		}

		// Then clone the response into the response writer, and voila.
		if err := resp.Clone(w.Message()); err != nil {
			coap.Abort(w)
			return
		}

		return
	}

	clientAddr, ok := qs["clientAddr"]
	if ok {
		clientConn, err := r.getClientConnection(clientAddr)
		if err != nil {
			log.Println(fmt.Sprintf("no connection for %s client", clientAddr))
			coap.Abort(w)
			return
		}

		srvMsg := msg.Message
		resp, err := r.redirectToClient(clientConn, srvMsg)
		if err != nil {
			w.SetResponse(
				codes.InternalServerError,
				message.TextPlain,
				bytes.NewReader([]byte(err.Error())),
			)
			return
		}

		// Then clone the response into the response writer, and voila.
		if err := resp.Clone(w.Message()); err != nil {
			coap.Abort(w)
			return
		}

		return
	}

	log.Println("unknown request")
	w.SetResponse(
		codes.NotImplemented,
		message.TextPlain,
		bytes.NewReader([]byte("unknown request")),
	)
}

func (r *Redirector) getOrRegisterClientConnection(addr string, w mux.ResponseWriter) mux.Conn {
	clientConn, err := r.getClientConnection(addr)
	if err == nil {
		return clientConn
	}

	log.Println(fmt.Sprintf("new client connection: %s", addr))
	r.clients[addr] = w.Conn()
	return w.Conn()
}

func (r *Redirector) getClientConnection(addr string) (mux.Conn, error) {
	if client, ok := r.clients[addr]; ok {
		return client, nil
	}

	return nil, ErrClientNotKnown
}

func (r *Redirector) getServerConnection(key string) (mux.Conn, error) {
	if conn, ok := r.servers[key]; ok {
		return conn, nil
	}

	node, err := r.ring.GetNode([]byte(key))
	if err != nil {
		log.Println("could not retrieve the node", err)
		return nil, err
	}

	peer, err := net.ResolveUDPAddr("udp", node.Addr.String()+":10000")
	if err != nil {
		log.Println("could resolve server addr", err)
		return nil, err
	}

	conn, err := r.coapSrv.NewConn(peer)
	if err != nil {
		log.Println("could resolve server addr", err)
		return nil, err
	}

	r.servers[key] = conn
	return conn, nil
}

func (r *Redirector) redirectToServer(srvConn mux.Conn, clientConn mux.Conn, clientMsg *pool.Message) (*pool.Message, error) {
	srvMsg := srvConn.AcquireMessage(context.Background())
	if err := clientMsg.Clone(srvMsg); err != nil {
		return nil, err
	}

	// adding identifier of client request
	srvMsg.AddQuery(fmt.Sprintf("clientAddr=%s", clientConn.RemoteAddr().String()))

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

func NewRedirector() {

}
