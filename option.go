package krpc

import "github.com/k-si/krpc/codec"

const Magic = 0x31415926 // 4 bytes

// Option is used to determine the protocol, a binary message like:
// {Magic: xxx, CodecType: xxx} | head | args/reply  | head | args/reply  | ...
// client encode Option by json, and encode (head,body) by Option.CodeType
// similarly, server can decode Option by json, and decode (head,body) by Option.CodeType
// todo: warn: this message solution has a bug, json.NewDecode will read all bytes, so that some (head,body) may be missing
type Option struct {
	Magic    int
	CodeType codec.CodeType
}

var DefaultOption = &Option{
	Magic:    Magic,
	CodeType: codec.Gob,
}
