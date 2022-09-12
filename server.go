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
	"sync/atomic"
)

var DefaultServer = NewServer()

type Server struct {
}

func NewServer() *Server {
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
	var mu sync.Mutex // ensure that the order of processing requests is not chaotic
	for {
		req, err := s.readRequest(cc)
		log.Println("request:", req.h, req.args.Elem())
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

// getCodec return a codec, and decode opt{Magic: xxx, CodecType: xxx}
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

// =================================== handle request ===================================

type request struct {
	h           *codec.Head
	args, reply reflect.Value
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
	req.args = reflect.New(reflect.TypeOf(""))
	if err = cc.ReadBody(req.args.Interface()); err != nil {
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

// =================================== service register ===================================

type methodType struct {
	method    reflect.Method
	argsType  reflect.Type
	replyType reflect.Type
	numCalls  uint64
}

func (mt *methodType) NumCalls() uint64 {
	return atomic.LoadUint64(&mt.numCalls)
}

func (mt *methodType) newArgv() reflect.Value {
	var argv reflect.Value

	if mt.argsType.Kind() == reflect.Ptr {
		argv = reflect.New(mt.argsType.Elem())
	} else {
		argv = reflect.New(mt.argsType).Elem()
	}

	return argv
}

func (mt *methodType) newReply() reflect.Value {
	replyv := reflect.New(mt.replyType.Elem()) // reply must be pointer

	switch mt.replyType.Elem().Kind() {
	case reflect.Map:
		replyv.Elem().Set(reflect.MakeMap(mt.replyType.Elem()))
	case reflect.Slice:
		replyv.Elem().Set(reflect.MakeSlice(mt.replyType.Elem(), 0, 0))
	}

	return replyv
}

type service struct {
	name     string
	typ      reflect.Type
	receiver reflect.Value
	method   map[string]*methodType
}
