package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"reflect"
	"sync"

	"github.com/spade69/carnival/codec"
)

const MagicNumber = 0x3bef5c

type Option struct {
	// to mark it 's carnival rpc request
	MagicNumber int
	CodecType   codec.Type
}

var DefaultOption = &Option{
	MagicNumber: MagicNumber,
	CodecType:   codec.GodType,
}

type Server struct{}

// NewServer is default instance of *Server
func NewServer() *Server {
	return &Server{}
}

// request store all info of a call
type request struct {
	h            *codec.Header
	argv, replyv reflect.Value
}

var DefaultServer = NewServer()

func ParseOptions(opts ...*Option) (*Option, error) {
	// if opts is nil or pass nil as parameter
	if len(opts) == 0 || opts[0] == nil {
		return DefaultOption, nil
	}
	if len(opts) != 1 {
		return nil, errors.New("number of options is more than 1")
	}
	opt := opts[0]
	opt.MagicNumber = DefaultOption.MagicNumber
	if opt.CodecType == "" {
		opt.CodecType = DefaultOption.CodecType
	}
	return opt, nil
}

// // Accept accepts connections on the listener and serves requests

func (server *Server) Accept(lis net.Listener) {

	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Println("rpc server: accept error:", err)

			return
		}
		go server.ServeConn(conn)
	}
}

func (server *Server) ServeConn(conn io.ReadWriteCloser) {
	defer func() {
		_ = conn.Close()
	}()
	var opt Option
	if err := json.NewDecoder(conn).Decode(&opt); err != nil {
		log.Println("rpc server : options error:", err)
		return
	}
	if opt.MagicNumber != MagicNumber {
		log.Printf("rpc server: invliad magin number %x", opt.MagicNumber)
		return
	}
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		log.Printf("rpc server: invalid codec type %s", opt.CodecType)
		return
	}

	server.serveCodec(f(conn))
}

// invalidRequest is a placeholder for response argv when error occurs
var invalidRequest = struct{}{}

func (server *Server) serveCodec(cc codec.Codec) {
	// make sure send a complete response
	sending := new(sync.Mutex)
	// wait until all request handled
	wg := new(sync.WaitGroup)
	for {
		// 1. first readRequest
		req, err := server.readRequest(cc)
		// 2. send reponse
		if err != nil {
			if req == nil {
				break
			}
			req.h.Error = err.Error()
			// reuse sync.Mutex
			server.sendReponse(cc, req.h, invalidRequest, sending)
			continue
		}
		// 3. handleRequest
		wg.Add(1)
		// reuse sync.Mutex, using waitgourp to concurrent
		go server.handleRequest(cc, req, sending, wg)
	}
	wg.Wait()
	_ = cc.Close()
}

// process read request
func (server *Server) readRequest(cc codec.Codec) (*request, error) {
	h, err := server.readRequestHeader(cc)
	if err != nil {
		return nil, err
	}
	req := &request{h: h}
	// TODO: now we dont know type of request argv, suppose it string
	req.argv = reflect.New(reflect.TypeOf(""))
	if err = cc.ReadBody(req.argv.Interface()); err != nil {
		log.Println("rpc server: read argv err:", err)
	}
	return req, nil
}

// handle request
func (server *Server) readRequestHeader(cc codec.Codec) (*codec.Header, error) {
	var h codec.Header
	if err := cc.ReadHeader(&h); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			log.Println("rpc server: read request header err", err)
		}
		return nil, err
	}
	return &h, nil
}

// send response
func (server *Server) sendReponse(cc codec.Codec, h *codec.Header, body interface{}, sending *sync.Mutex) {
	sending.Lock()
	defer sending.Unlock()
	// write reponse concurrent safe
	if err := cc.Write(h, body); err != nil {
		log.Println("rpc server: write response error:", err)
	}
}

// handle Request use goroutine to concurrent process request
func (server *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup) {
	//
	// print and send hello msg to client
	defer wg.Done()
	// log out request
	log.Println(req.h, req.argv.Elem())
	req.replyv = reflect.ValueOf(fmt.Sprintf("rpc resp %d", req.h.Seq))
	server.sendReponse(cc, req.h, req.replyv.Interface(), sending)
}
