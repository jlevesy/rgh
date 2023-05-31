package coap

import (
	"bytes"
	"log"
	"net"

	"github.com/plgd-dev/go-coap/v3/message"
	"github.com/plgd-dev/go-coap/v3/message/codes"
	"github.com/plgd-dev/go-coap/v3/mux"
	"github.com/plgd-dev/go-coap/v3/udp/server"
)

func Forward(addr string, w mux.ResponseWriter, req *mux.Message, srv *server.Server) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		log.Println("could not resolve udp addr", err)
		w.SetResponse(
			codes.InternalServerError,
			message.TextPlain,
			bytes.NewReader([]byte("something went wrong")),
		)
		return

	}
	downstream, err := srv.NewConn(udpAddr)
	if err != nil {
		log.Println("could not dial downstream", err)
		w.SetResponse(
			codes.InternalServerError,
			message.TextPlain,
			bytes.NewReader([]byte("something went wrong")),
		)
		return
	}
	defer downstream.Close()

	// Forward the message.
	resp, err := downstream.Do(req.Message)
	if err != nil {
		log.Println("could not forward message", err)
		Abort(w)
		return
	}

	// Then clone the response into the response writer, and voila.
	if err := resp.Clone(w.Message()); err != nil {
		Abort(w)
		return
	}
}

func Abort(w mux.ResponseWriter) {
	w.SetResponse(
		codes.InternalServerError,
		message.TextPlain,
		bytes.NewReader([]byte("something went wrong")),
	)
}
