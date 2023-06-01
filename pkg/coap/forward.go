package coap

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"

	"github.com/jlevesy/rgh/pkg/memberring"
	"github.com/plgd-dev/go-coap/v3/message"
	"github.com/plgd-dev/go-coap/v3/message/codes"
	"github.com/plgd-dev/go-coap/v3/mux"
	"github.com/plgd-dev/go-coap/v3/udp/server"
)

const upstreamKeyOptionID message.OptionID = 204

type Forwarder struct {
	mu       sync.RWMutex
	connMaps map[string]mux.Conn
	server   *server.Server
	ring     memberring.Ring
}

func NewForwarder(ring memberring.Ring) *Forwarder {
	return &Forwarder{
		connMaps: make(map[string]mux.Conn),
		ring:     ring,
	}
}

func (f *Forwarder) SetServer(s *server.Server) {
	f.server = s
}

func (f *Forwarder) ServeCOAP(w mux.ResponseWriter, req *mux.Message) {
	if req.Message.HasOption(upstreamKeyOptionID) {
		// OUH I RECEIVED BACK A MESSAGE.
		originalAddress, err := req.Message.Options().GetString(upstreamKeyOptionID)
		if err != nil {
			log.Println("Could not get mid opt", err)
			abort(w)
			return
		}

		upstream, ok := f.readUpstream(originalAddress)
		if !ok {
			fmt.Println("not upstream lol")
			abort(w)
			return
		}

		resp, err := upstream.Do(req.Message)
		if err != nil {
			fmt.Println("Can't send back message", err)
			abort(w)
			return
		}

		if err := resp.Clone(w.Message()); err != nil {
			abort(w)
			return
		}

		return
	}

	rawQueries, err := req.Message.Queries()
	if err != nil {
		log.Println("could retrieve queries", rawQueries)
		abort(w)
		return
	}

	qs := parseQueries(rawQueries)

	key, ok := qs["key"]
	if !ok {
		log.Println("could retrieve key")
		abort(w)
		return
	}

	srv, err := f.ring.GetNode([]byte(key))
	if err != nil {
		log.Println("could not retrieve the node", err)
		abort(w)
		return
	}

	fmt.Println("Routing query to node", srv.Name)

	// TODO no hardcoded port, bad
	udpAddr, err := net.ResolveUDPAddr("udp", srv.Addr.String()+":10000")
	if err != nil {
		log.Println("could not resolve udp addr", err)
		abort(w)
		return
	}

	downstream, err := f.server.NewConn(udpAddr)
	if err != nil {
		log.Println("could not dial downstream", err)
		abort(w)
		return
	}
	defer downstream.Close()

	upstreamKey := key + w.Conn().RemoteAddr().String()

	f.storeUpstream(upstreamKey, w.Conn())
	defer f.clearUpstream(upstreamKey)

	req.Message.AddOptionString(upstreamKeyOptionID, upstreamKey)

	// Forward the message.
	resp, err := downstream.Do(req.Message)
	if err != nil {
		log.Println("could not forward message", err)
		abort(w)
		return
	}
	// Then clone the response into the response writer, and voila.
	if err := resp.Clone(w.Message()); err != nil {
		abort(w)
		return
	}
}

func (f *Forwarder) storeUpstream(mid string, conn mux.Conn) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.connMaps[mid] = conn
}

func (f *Forwarder) clearUpstream(mid string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	delete(f.connMaps, mid)
}

func (f *Forwarder) readUpstream(mid string) (mux.Conn, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	conn, ok := f.connMaps[mid]
	return conn, ok
}

func abort(w mux.ResponseWriter) {
	w.SetResponse(
		codes.InternalServerError,
		message.TextPlain,
		bytes.NewReader([]byte("something went wrong")),
	)
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
