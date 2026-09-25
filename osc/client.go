package osc

import (
	"fmt"
	"net"
	"sync"
)

////
// Client
////

// Client enables you to send OSC packets. It sends OSC messages and bundles to
// the given IP address and port.
type Client struct {
	ip     string
	port   int
	laddr  *net.UDPAddr
	conn   *net.UDPConn
	server *Server
	mtx    sync.Mutex
}

// NewClient creates a new OSC client. The Client is used to send OSC
// messages and OSC bundles over an UDP network connection. The `ip` argument
// specifies the IP address and `port` defines the target port where the
// messages and bundles will be send to.
func NewClient(ip string, port int) *Client {
	return &Client{
		ip:     ip,
		port:   port,
		laddr:  nil,
		server: NewServer(),
	}
}

// IP returns the IP address.
func (c *Client) IP() string { return c.ip }

// SetIP sets a new IP address.
func (c *Client) SetIP(ip string) { c.ip = ip }

// Port returns the port.
func (c *Client) Port() int { return c.port }

// SetPort sets a new port.
func (c *Client) SetPort(port int) { c.port = port }

// SetConnection sets the connection to use
func (c *Client) SetConnection(conn *net.UDPConn) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.conn = conn
	c.server.SetConnection(c.conn)
}

// Connection returns the current connection
func (c *Client) Connection() *net.UDPConn {
	return c.conn
}

// SetDispatcher sets the dispatcher to use to handle responses.
func (c *Client) SetDispatcher(d Dispatcher) {
	c.server.Dispatcher = d
}

// SetLocalAddr sets the local address.
func (c *Client) SetLocalAddr(ip string, port int) error {
	laddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		return err
	}
	c.laddr = laddr
	return nil
}

// LocalAddr gets the local address.
func (c *Client) LocalAddr() *net.UDPAddr { return c.laddr }

// LocalIP gets the local listening IP address
func (c *Client) LocalIP() string { return c.laddr.IP.String() }

// LocalPort gets the local listening IP address
func (c *Client) LocalPort() int { return c.laddr.Port }

// Connected returns the client connected state
func (c *Client) Connected() bool {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.conn != nil
}

// Connect explicitly connects to the target address. Normally you don't have to call
// this, as both Send and ListenAndServe establish the connection if necessary.
func (c *Client) Connect() error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.conn != nil {
		return nil // already connected
	}
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", c.ip, c.port))
	if err != nil {
		return err
	}
	if addr.IP.IsLoopback() { // Workaround: weird addresses like 127.0.1.1 don't work
		addr.IP = net.ParseIP("127.0.0.1")
	}
	c.conn, err = net.DialUDP("udp", c.laddr, addr)
	if err != nil {
		return err
	}
	c.server.SetConnection(c.conn)
	return nil
}

// Close closes the current connection.
func (c *Client) Close() {
	if c.Connected() {
		c.conn.Close()
		c.conn = nil
	}
}

// Send sends an OSC Bundle or an OSC Message. If no connection exists, one will be established.
func (c *Client) Send(packet Packet) error {
	if !c.Connected() {
		err := c.Connect()
		if err != nil {
			return err
		}
	}
	data, err := packet.MarshalBinary()
	if err != nil {
		return err
	}

	if _, err = c.conn.Write(data); err != nil {
		return err
	}
	return nil
}

// ListenAndServe starts the listening and dispatching loop. It listens on the same port, that
// was established by the client. You only need to call this if you expect responses from the server.
// This function only returns when there is an error, so better put it into a go routine.
func (c *Client) ListenAndServe() error {
	if !c.Connected() {
		err := c.Connect()
		if err != nil {
			return err
		}
	}
	if c.server.Dispatcher == nil {
		c.server.Dispatcher = NewStandardDispatcher()
	}

	return c.server.Serve()
}

// Ready returns a chan that blocks until ListenAndServe is ready and listening
func (c *Client) Ready() <-chan struct{} {
	return c.server.Ready()
}
