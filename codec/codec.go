package codec

import "io"

type Header struct {
	// Service.Method name , mapping struct and func
	ServiceMethod string // format "service.method"
	// can be distinguish as request id
	Seq   uint64 // sequence number chosen by client
	Error string
}

// Encoding and Decoding interface
// used to implement different Codec instance: GobCodeC or JSONCodec ....
type Codec interface {
	io.Closer
	ReadHeader(*Header) error
	ReadBody(interface{}) error
	Write(*Header, interface{}) error
}

// Construction Func for init CodeC instance
type NewCodecFunc func(io.ReadWriteCloser) Codec
type Type string

const (
	// Gob
	GodType Type = "application/gob"
	// json
	JsonType Type = "application/json"
)

var NewCodecFuncMap map[Type]NewCodecFunc

func init() {
	NewCodecFuncMap = make(map[Type]NewCodecFunc)
	// client or server and get Construction Function by passing Type  as FuncMap key
	NewCodecFuncMap[GodType] = NewGobCodec
}
