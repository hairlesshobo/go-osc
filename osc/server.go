package osc

import (
	"bufio"
	"bytes"
	"errors"
	"net"
	"time"
)

////
// Server
////

// Server represents an OSC server. The server listens on Address and Port for
// incoming OSC packets and bundles.
type Server struct {
	Addr        string
	Dispatcher  Dispatcher
	ReadTimeout time.Duration
	conn        net.PacketConn
	close       func() error
	ready       chan struct{}
}

func NewServer() *Server {
	return &Server{
		ready: make(chan struct{}),
	}
}

// ListenAndServe retrieves incoming OSC packets and dispatches the retrieved
// OSC packets.
func (s *Server) ListenAndServe() error {
	defer s.CloseConnection()

	if s.Dispatcher == nil {
		s.Dispatcher = NewStandardDispatcher()
	}

	if s.conn == nil {
		s.Listen()
	}

	return s.Serve()
}

// Listen creates the listening port and sets up the connection.
func (s *Server) Listen() error {
	ln, err := net.ListenPacket("udp", s.Addr)
	if err != nil {
		return err
	}
	s.conn = ln

	s.close = ln.Close
	return nil
}

// SetConnection sets the connection to use for the server. This is for a case
// where you created the connection manually instead of using ListenAndServer.
func (s *Server) SetConnection(c net.PacketConn) {
	s.conn = c
}

// Serve retrieves incoming OSC packets from the given connection and dispatches
// retrieved OSC packets. If something goes wrong an error is returned.
func (s *Server) Serve() error {
	close(s.ready)

	var tempDelay time.Duration
	for {
		msg, err := s.readFromConnection()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				time.Sleep(tempDelay)
				continue
			}
			return err
		}
		tempDelay = 0

		go s.Dispatcher.Dispatch(msg)
	}
}

// Ready indicates that the server is started and listening for packets
func (s *Server) Ready() <-chan struct{} {
	return s.ready
}

// CloseConnection forcibly closes a server's connection.
//
// This causes a "use of closed network connection" error the next time the
// server attempts to read from the connection.
func (s *Server) CloseConnection() error {
	if s.close == nil {
		return nil
	}

	return s.close()
}

// ReceivePacket listens for incoming OSC packets and returns the packet if one is received.
func (s *Server) ReceivePacket() (Packet, error) {
	return s.readFromConnection()
}

// readFromConnection retrieves OSC packets.
func (s *Server) readFromConnection() (Packet, error) {
	if s.conn == nil {
		return nil, errors.New("not connected")
	}
	if s.ReadTimeout != 0 {
		if err := s.conn.SetReadDeadline(time.Now().Add(s.ReadTimeout)); err != nil {
			return nil, err
		}
	}

	data := make([]byte, 65535)
	n, src, err := s.conn.ReadFrom(data)
	if err != nil {
		return nil, err
	}

	var start int
	p, err := readPacket(bufio.NewReader(bytes.NewBuffer(data)), &start, n, src)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// SendTo sends a message to the given address. The sender address will be the address and
// port the server is listening on.
func (s *Server) SendTo(packet Packet, addr net.Addr) error {
	data, err := packet.MarshalBinary()
	if err != nil {
		return err
	}

	if _, err = s.conn.WriteTo(data, addr); err != nil {
		return err
	}
	return nil
}
