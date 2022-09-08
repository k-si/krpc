package krpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/k-si/krpc/codec"
	"net"
	"sync"
)

// Call contains all the parameters required for one request
type Call struct {
	Seq    uint64
	Method string
	Argv   interface{}
	Reply  interface{}
	Error  error
	Done   chan *Call
}

func (c *Call) done() {
	c.Done <- c
}

type Client struct {
	seq      uint64
	closing  bool       // client close
	shutdown bool       // server shutdown
	sending  sync.Mutex // lock cli.send
	mu       sync.Mutex // lock closing, shutdown, pending, seq
	cc       codec.Codec
	opt      *Option
	pending  map[uint64]*Call
}

func Dial(network, address string, opt *Option) (*Client, error) {
	if opt == nil {
		opt = DefaultOption
	}

	newCodec := codec.NewCodecMap[opt.CodeType]
	if newCodec == nil {
		return nil, errors.New("can not get codec type from option")
	}

	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}

	// before request, send json string of option first
	enc := json.NewEncoder(conn)
	if err := enc.Encode(opt); err != nil {
		_ = conn.Close()
		return nil, err
	}

	cli := &Client{
		seq:      1,
		closing:  false,
		shutdown: false,
		sending:  sync.Mutex{},
		mu:       sync.Mutex{},
		cc:       newCodec(conn),
		opt:      opt,
		pending:  make(map[uint64]*Call),
	}
	go cli.receive()

	return cli, nil
}

func (cli *Client) Call(method string, argv, reply interface{}) error {
	call := <-cli.Go(method, argv, reply, make(chan *Call, 1)).Done
	return call.Error
}

func (cli *Client) Go(method string, argv, reply interface{}, done chan *Call) *Call {
	if done == nil || cap(done) == 1 {
		done = make(chan *Call, 10)
	}

	call := &Call{
		Method: method,
		Argv:   argv,
		Reply:  reply,
		Done:   done,
	}
	go cli.send(call)

	return call
}

func (cli *Client) Close() error {
	cli.mu.Lock()
	defer cli.mu.Unlock()

	if cli.closing || cli.shutdown {
		return errors.New("close while client is closing or shutdown")
	}
	cli.closing = true

	return cli.cc.Close()
}

func (cli *Client) Unavailable() bool {
	cli.mu.Lock()
	defer cli.mu.Unlock()

	return cli.closing || cli.shutdown
}

func (cli *Client) send(call *Call) {
	cli.sending.Lock()
	defer cli.sending.Unlock()

	seq := cli.registerCall(call)
	head := &codec.Head{
		Method: call.Method,
		Seq:    seq,
		Error:  "",
	}

	if err := cli.cc.Write(head, call.Argv); err != nil {
		c := cli.popCall(seq)
		if c != nil {
			c.Error = err
			c.done()
		}
	}
}

func (cli *Client) receive() {
	var err error

	for err == nil {
		head := &codec.Head{}
		if err = cli.cc.ReadHead(head); err != nil {
			break
		}

		call := cli.popCall(head.Seq)

		switch {

		// if cc write message happened error, will pop this call
		case call == nil:
			err = cli.cc.ReadBody(nil)
		case head.Error != "":
			call.Error = fmt.Errorf(head.Error)
			call.done()
		default:
			if err = cli.cc.ReadBody(call.Reply); err != nil {
				call.Error = err
			}
			call.done()
		}
	}

	// if happened error, terminate all calls
	cli.terminateCalls(err)
}

func (cli *Client) registerCall(call *Call) uint64 {
	cli.mu.Lock()
	defer cli.mu.Unlock()

	// In the above, call.seq is always 0
	// call.seq need to be corrected with cli.seq
	call.Seq = cli.seq
	cli.seq++

	cli.pending[call.Seq] = call

	return call.Seq
}

func (cli *Client) popCall(req uint64) *Call {
	cli.mu.Lock()
	defer cli.mu.Unlock()

	call := cli.pending[req]
	delete(cli.pending, req)

	return call
}

func (cli *Client) terminateCalls(err error) {
	cli.sending.Lock()
	defer cli.sending.Unlock()
	cli.mu.Lock()
	defer cli.mu.Unlock()

	cli.shutdown = true

	for _, call := range cli.pending {
		call.Error = err
		call.done()
	}
}
