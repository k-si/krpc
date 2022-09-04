package codec

import (
	"bufio"
	"encoding/gob"
	"io"
	"log"
)

var _ Codec = (*GobCodec)(nil)

type GobCodec struct {
	conn io.ReadWriteCloser
	buf  *bufio.Writer // why writer
	enc  *gob.Encoder
	dec  *gob.Decoder
}

func NewGobCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn)
	return &GobCodec{
		conn: conn,
		buf:  buf,
		enc:  gob.NewEncoder(buf),
		dec:  gob.NewDecoder(conn),
	}
}

func (gob *GobCodec) ReadHead(head *Head) (err error) {
	err = gob.dec.Decode(head)
	return
}

func (gob *GobCodec) ReadBody(body interface{}) (err error) {
	err = gob.dec.Decode(body)
	return
}

func (gob *GobCodec) Write(head *Head, body interface{}) (err error) {
	defer func() {
		if err = gob.buf.Flush(); err != nil {
			log.Println("[krpc] gob flush err:", err)
			_ = gob.Close()
		}
	}()
	if err = gob.enc.Encode(head); err != nil {
		log.Println("[krpc] gob encode head err:", err)
		return
	}
	if err = gob.enc.Encode(body); err != nil {
		log.Println("[krpc] gob encode body err:", err)
		return
	}
	return
}

func (gob *GobCodec) Close() (err error) {
	err = gob.conn.Close()
	return
}
