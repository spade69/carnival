package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	"github.com/spade69/carnival/codec"
	"github.com/spade69/carnival/server"
)

// Call represent an active RPC

type Call struct {
	Seq           uint64
	ServiceMethod string
	// arguments to function
	Args interface{}
	// reply from function
	Reply interface{}
	//
	Error error
	//Strobes when call is complete
	Done chan *Call
}

// passing call instance to Done chan...
func (call *Call) done() {
	call.Done <- call
}

type Client struct {
	// codec for both clent and server
	cc codec.Codec
	// server option
	opt *server.Option
	// mutedx to protect request sending in order, prevent header confllict
	sending sync.Mutex
	// every request got this header.
	header codec.Header
	mu     sync.Mutex
	// sending request unique id
	seq uint64
	// store unfinished request, key is number, value is Call instance
	pending map[uint64]*Call
	// user has called close, closing or shutdown set to true then is unavialbel
	closing bool
	// server has told us to stop
	shutdown bool
}

var _ io.Closer = (*Client)(nil)

var ErrShutdown = errors.New("connection is shut down")

func NewClient(conn net.Conn, opt *server.Option) (*Client, error) {
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		err := fmt.Errorf("invalid codec type %s", opt.CodecType)
		log.Println("rpc client: codec error:", err)
		return nil, err
	}
	// send options with server
	if err := json.NewEncoder(conn).Encode(opt); err != nil {
		log.Println("rpc client: options error: ", err)
		_ = conn.Close()
		return nil, err
	}
	return newClientCodec(f(conn), opt), nil
}

func newClientCodec(cc codec.Codec, opt *server.Option) *Client {
	client := &Client{
		seq:     1,
		cc:      cc,
		opt:     opt,
		pending: make(map[uint64]*Call),
	}
	go client.recive()
	return client
}

func Dial(network, address string, opts ...*server.Option) (client *Client, err error) {
	opt, err := server.ParseOptions(opts...)
	if err != nil {
		return nil, err
	}
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}
	// close connection if client is nil
	defer func() {
		if client == nil {
			_ = conn.Close()
		}
	}()
	return NewClient(conn, opt)
}

func (client *Client) Close() error {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closing {
		return ErrShutdown
	}
	client.closing = true
	return client.cc.Close()
}

func (client *Client) registerCall(call *Call) (uint64, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closing || client.shutdown {
		return 0, ErrShutdown
	}
	// every call is an rpc
	call.Seq = client.seq
	// sent then set status to pending
	client.pending[call.Seq] = call
	// increment after every request
	client.seq++
	return client.seq, nil
}

func (client *Client) removeCall(seq uint64) *Call {
	client.mu.Lock()
	defer client.mu.Unlock()
	call := client.pending[seq]
	delete(client.pending, seq)
	return call
}

func (client *Client) terminateCalls(err error) {
	client.sending.Lock()
	defer client.sending.Unlock()
	client.mu.Lock()
	defer client.mu.Unlock()
	client.shutdown = true
	for _, call := range client.pending {
		call.Error = err
		call.done()
	}
}

func (client *Client) recive() {
	var err error
	for err == nil {
		var h codec.Header
		if err = client.cc.ReadHeader(&h); err != nil {
			break
		}
		call := client.removeCall(h.Seq)
		switch {
		case call == nil:
			// it usually means that write partially failed
			// and call was already removed
			err = client.cc.ReadBody(nil)
		case h.Error != "":
			call.Error = fmt.Errorf(h.Error)
			err = client.cc.ReadBody(nil)
			call.done()
		default:
			err = client.cc.ReadBody(call.Reply)
			if err != nil {
				call.Error = errors.New("reading body " + err.Error())
			}
			call.done()
		}
	}
	//  errors occurs and terminate pending calls
	client.terminateCalls(err)
}

// Call invokes the named function, waits for it to complete,
// and returns its error status.
func (client *Client) Call(serviceMethod string, args, reply interface{}) error {
	// blocking by call.Done chan ,wait for response
	call := <-client.Go(serviceMethod, args, reply, make(chan *Call, 1)).Done
	return call.Error
}

// Go is async api
func (client *Client) Go(serviceMethod string, args, reply interface{}, done chan *Call) *Call {
	call := &Call{
		ServiceMethod: serviceMethod,
		Args:          args,
		Reply:         reply,
		Done:          done,
	}
	client.send(call)
	return call
}

//
func (client *Client) send(call *Call) {
	// 	// make sure that the client will send a complete request
	client.sending.Lock()
	defer client.sending.Unlock()
	// register this call.
	seq, err := client.registerCall(call)
	if err != nil {
		call.Error = err
		call.done()
		return
	}
	// prepare request header
	client.header.ServiceMethod = call.ServiceMethod
	client.header.Seq = seq
	client.header.Error = ""
	// encode and send request using codecc.Write
	if err := client.cc.Write(&client.header, call.Args); err != nil {
		// err handle here
		call := client.removeCall(seq)
		// call may be nil, usually means write partially fail
		if call != nil {
			call.Error = err
			call.done()
		}
	}
}
