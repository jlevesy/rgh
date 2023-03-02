package coap

import (
	"bytes"
	"log"

	"github.com/plgd-dev/go-coap/v3/message"
	"github.com/plgd-dev/go-coap/v3/message/codes"
	"github.com/plgd-dev/go-coap/v3/mux"
	"github.com/plgd-dev/go-coap/v3/udp"
)

func Forward(addr string, w mux.ResponseWriter, req *mux.Message) {
	// Dial destination.
	// Very naive, could reuse conns or pool them.
	downstream, err := udp.Dial(addr)
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
