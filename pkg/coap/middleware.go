package coap

import (
	"context"
	"io"

	"github.com/plgd-dev/go-coap/v3/message"
	"github.com/plgd-dev/go-coap/v3/message/pool"
	"github.com/plgd-dev/go-coap/v3/mux"
)

func ForwardUpstreamKey(next mux.Handler) mux.Handler {
	return mux.HandlerFunc(func(w mux.ResponseWriter, r *mux.Message) {
		upstreamKey, err := r.Message.Options().GetString(upstreamKeyOptionID)
		if err != nil {
			abort(w)
			return
		}

		next.ServeCOAP(
			&setUpstreamKeyConn{
				ResponseWriter: w,
				upstreamKey:    upstreamKey,
			},
			r,
		)
	})
}

type setUpstreamKeyConn struct {
	mux.ResponseWriter

	upstreamKey string
}

func (rw *setUpstreamKeyConn) Conn() mux.Conn {
	return &upstreamKeyConn{
		Conn:        rw.ResponseWriter.Conn(),
		upstreamKey: rw.upstreamKey,
	}
}

type upstreamKeyConn struct {
	mux.Conn

	upstreamKey string
}

// TODO support all writing methods.
func (o *upstreamKeyConn) Post(ctx context.Context, path string, contentFormat message.MediaType, payload io.ReadSeeker, opts ...message.Option) (*pool.Message, error) {
	return o.Conn.Post(
		ctx,
		path,
		contentFormat,
		payload,
		append(
			opts,
			message.Option{
				ID:    upstreamKeyOptionID,
				Value: []byte(o.upstreamKey),
			},
		)...,
	)
}
