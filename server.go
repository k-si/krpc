package krpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/k-si/krpc/codec"
	"io"
	"log"
	"net"
	"reflect"
	"sync"
)

const Magic = 0x31415926 // 4 bytes

// Option is used to determine the protocol
// a binary message like:
// magic codeType               | head args/reply  | head args/reply  | ...
// {Magic: xxx, CodecType: xxx} | gob or other     | gob or other     | ...
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

var DefaultServer = New()

type Server struct {
}

func New() *Server {
	return &Server{}
}

func (s *Server) Run(lis net.Listener) {
	DefaultServer.accept(lis)
}

func (s *Server) accept(lis net.Listener) {
	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Println("[krpc] accept err:", err)
			continue
		}
		log.Println("[krpc] accept conn, local:", conn.LocalAddr(), "remote:", conn.RemoteAddr())
		go s.serve(conn)
	}
}

var invalidRequest = struct{}{}

func (s *Server) serve(conn io.ReadWriteCloser) {
	cc, err := s.getCodec(conn)
	log.Println("[krpc] get codec success")
	if err != nil {
		return
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	for {
		req, err := s.readRequest(cc)
		log.Println("request:", req.h, req.argv.Elem())
		if err != nil {
			if req == nil {
				req = &request{h: new(codec.Head)}
			}
			req.h.Error = err.Error()
			s.writeResponse(cc, req.h, invalidRequest, &mu)
			break
		}
		wg.Add(1)
		go s.handleRequest(cc, req, &wg, &mu)
	}
	wg.Wait()
	_ = conn.Close()
}

func (s *Server) getCodec(conn io.ReadWriteCloser) (codec.Codec, error) {
	var opt Option

	// parse Option from conn, and create codec
	if err := json.NewDecoder(conn).Decode(&opt); err != nil {
		return nil, err
	}
	if opt.Magic != Magic {
		return nil, errors.New("magic number not match")
	}
	newCodec := codec.NewCodecMap[opt.CodeType]
	if newCodec == nil {
		return nil, errors.New("no function that create a codec")
	}

	return newCodec(conn), nil
}

type request struct {
	h           *codec.Head
	argv, reply reflect.Value
}

func (s *Server) readRequestHead(cc codec.Codec) (*codec.Head, error) {
	var h codec.Head
	if err := cc.ReadHead(&h); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			log.Println("[krpc] read request head err:", err)
		}
		return nil, err
	}
	return &h, nil
}

func (s *Server) readRequest(cc codec.Codec) (*request, error) {
	head, err := s.readRequestHead(cc)
	if err != nil {
		return nil, err
	}
	var req = &request{
		h: head,
	}
	// todo
	req.argv = reflect.New(reflect.TypeOf(""))
	if err = cc.ReadBody(req.argv.Interface()); err != nil {
		log.Println("[krpc] read request body err:", err)
		return nil, err
	}
	return req, nil
}

func (s *Server) writeResponse(cc codec.Codec, head *codec.Head, body interface{}, mu *sync.Mutex) {
	mu.Lock()
	defer mu.Unlock()
	if err := cc.Write(head, body); err != nil {
		log.Println("[krpc] write response err:", err)
	}
}

func (s *Server) handleRequest(cc codec.Codec, req *request, wg *sync.WaitGroup, mu *sync.Mutex) {
	defer wg.Done()
	// todo
	req.reply = reflect.ValueOf(fmt.Sprintf("krpc response %d", req.h.Seq))
	s.writeResponse(cc, req.h, req.reply.Interface(), mu)
}
