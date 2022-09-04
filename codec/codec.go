package codec

import "io"

// Head is an abstraction of rpc request parameters
// a rpc request like
// err := cli.Call("{service}.{method}", args, reply)
// head contains "{service}.{method}" and err
type Head struct {
	Method string // "{service}.{method}"
	Seq    uint64 // sequence number
	Error  string // rpc response error
}

type Codec interface {
	io.Closer
	ReadHead(*Head) error
	ReadBody(interface{}) error
	Write(*Head, interface{}) error
}

type CodeType string

const (
	Gob CodeType = "application/gob"
)

// NewCodec is used to create a codec
type NewCodec func(io.ReadWriteCloser) Codec

// NewCodecMap stores some functions for creating codecs
// therefor, we can easily obtain a NewCodec like NewFuncMap[Gob],
// instead of obtaining the NewCodec through switch case like
// switch (CodeType) { case Gob: return NewGobCodec}
var NewCodecMap = make(map[CodeType]NewCodec)

func init() {
	NewCodecMap[Gob] = NewGobCodec
}
