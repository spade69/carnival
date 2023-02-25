package codec

import (
	"bufio"
	"encoding/gob"
	"fmt"
	"io"
)

type GobCodec struct {
	// ReadWriteCloser is the interface that groups the basic Read, Write and Close methods.
	Conn io.ReadWriteCloser
	Buf  *bufio.Writer
	Dec  *gob.Decoder
	Enc  *gob.Encoder
}

// make sure a GobCodec implements all Method of Codec
var _ Codec = (*GobCodec)(nil)

func NewGobCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn)
	return &GobCodec{
		// TCP or Unix create Socket
		Conn: conn,
		// buffer io , non-blocking
		Buf: buf,
		Dec: gob.NewDecoder(conn),
		Enc: gob.NewEncoder(buf),
	}
}

func (c *GobCodec) ReadHeader(h *Header) error {
	// Decode read data from input stream and store it in the data
	// represent by Header, value must be a pointer to the correct type
	return c.Dec.Decode(h)
}

func (c *GobCodec) ReadBody(body interface{}) error {
	// represent by body , it can be any data structure, no specific type
	return c.Dec.Decode(body)
}

func (c GobCodec) Write(h *Header, body interface{}) (err error) {
	defer func() {
		// flush to writer
		err := c.Buf.Flush()
		if err != nil {
			_ = c.Close()
		}
	}()
	if err := c.Enc.Encode(h); err != nil {
		fmt.Println("rpc codec: gob error encoding header:", err)
		return err
	}
	if err := c.Enc.Encode(body); err != nil {
		fmt.Println("rpc codec: gob error encoding body:", err)
		return err
	}
	return nil
}

func (c *GobCodec) Close() error {
	return c.Conn.Close()
}
