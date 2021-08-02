package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Jonchun/xmppeeker/xmpp"
)

var errStreamOpened = errors.New("stream successfully opened")

// ProxyConfig contains config information required for a Proxy
type ProxyConfig struct {
	Address        string
	Domain         string
	ConnectTimeout int
	LogPath        string
	LogTimeFormat  string
	FileTimeFormat string
	TLSConfig      *tls.Config
}

// Proxy acts as a forwarding agent for XML Elements and overwrites specific fields when necessary.
type Proxy struct {
	Config         *ProxyConfig
	client         connStruct
	server         connStruct
	logName        string
	saslSuccess    bool
	tlsProceedChan chan struct{}
}

// connStruct is a logical grouping containing structs necessary for client and server connections
type connStruct struct {
	Conn           net.Conn
	Decoder        *xmpp.Decoder
	ForwardHandler xmpp.Handler
	ReadWriter     io.ReadWriter
	Router         *xmpp.Router
	Stream         *xmpp.Stream
}

// NewProxy accepts a client connection and a ProxyConfig and returns a new Proxy
func NewProxy(clientConn net.Conn, config *ProxyConfig) *Proxy {
	p := &Proxy{
		Config: config,
	}
	p.setLogName(clientConn)
	p.SetClientConn(clientConn)

	// Setup default forwarding handlers
	p.client.ForwardHandler = xmpp.HandlerFunc(func(e xmpp.Element) error {
		return p.SendClient(e.XML())
	})
	p.server.ForwardHandler = xmpp.HandlerFunc(func(e xmpp.Element) error {
		return p.SendServer(e.XML())
	})

	p.setupClientRouter()
	p.setupServerRouter()

	return p
}

func (p *Proxy) Close() error {
	errorMsg := "proxy close error"
	var err error
	if p.client.Conn != nil {
		if err = p.client.Conn.Close(); err != nil {
			errorMsg = fmt.Sprintf("%s: %s", errorMsg, err.Error())
		}
	}
	if p.server.Conn != nil {
		if err = p.server.Conn.Close(); err != nil {
			errorMsg = fmt.Sprintf("%s: %s", errorMsg, err.Error())
		}
	}
	// err will be non-nil if we had at least one error.
	if err != nil {
		return errors.New(errorMsg)
	}
	return nil
}

// Run will connect the client connection to a backend server connection.
func (p *Proxy) Run() error {
	defer p.Close()

	if err := p.ConnectToServer(); err != nil {
		return err
	}

	doneChan := make(chan struct{})
	go p.runClientRouter(doneChan)
	go p.runServerRouter(doneChan)

	// Block until at least one of the routers completes
	<-doneChan
	return nil
}

// SetClientConn sets the connection from the client
func (p *Proxy) SetClientConn(conn net.Conn) error {
	p.client.Conn = conn

	logFile := fmt.Sprintf("%s.C2P.log", p.logName)
	// If the file doesn't exist, create it, or append to the file
	f, err := appFS.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("error opening log file %s: %s", logFile, err)
	}
	config := &StreamLoggerConfig{
		Src:         conn,
		Dest:        f,
		TimeFormat:  p.Config.LogTimeFormat,
		ReadPrefix:  []byte(" C->P "),
		ReadSuffix:  []byte("\n"),
		WritePrefix: []byte(" P->C "),
		WriteSuffix: []byte("\n"),
	}
	p.client.ReadWriter = NewStreamLogger(config)
	p.client.Decoder = xmpp.NewDecoder(p.client.ReadWriter)
	return nil
}

// SetServerConn sets the connection to the server
func (p *Proxy) SetServerConn(conn net.Conn) error {
	p.server.Conn = conn

	logFile := fmt.Sprintf("%s.P2S.log", p.logName)
	// If the file doesn't exist, create it, or append to the file
	f, err := appFS.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("error opening log file %s: %s", logFile, err)
	}
	config := &StreamLoggerConfig{
		Src:         conn,
		Dest:        f,
		TimeFormat:  p.Config.LogTimeFormat,
		ReadPrefix:  []byte(" S->P "),
		ReadSuffix:  []byte("\n"),
		WritePrefix: []byte(" P->S "),
		WriteSuffix: []byte("\n"),
	}
	p.server.ReadWriter = NewStreamLogger(config)
	p.server.Decoder = xmpp.NewDecoder(p.server.ReadWriter)
	return nil
}

// ConnectToServer opens a TCP connection with the server
func (p *Proxy) ConnectToServer() error {
	if p.server.Conn == nil {
		connectTimeout := p.Config.ConnectTimeout
		if connectTimeout == 0 {
			connectTimeout = 10
		}
		conn, err := net.DialTimeout("tcp", p.Config.Address, time.Duration(connectTimeout)*time.Second)
		if err != nil {
			return err
		}

		return p.SetServerConn(conn)
	}
	return nil
}

// SendServer sends a string to the connection with the client
func (p *Proxy) SendClient(str string) (err error) {
	if p.client.ReadWriter != nil {
		_, err = fmt.Fprint(p.client.ReadWriter, str)
	}
	return err
}

// SendServer sends a string to the connection with the server
func (p *Proxy) SendServer(str string) (err error) {
	if p.server.ReadWriter != nil {
		_, err = fmt.Fprint(p.server.ReadWriter, str)
	}
	return err
}

// StartTLSWithClient upgrades the connection with the client
func (p *Proxy) StartTLSWithClient() error {
	// When communicating with the client, the proxy is acting as the TLS server.
	tlsConn := tls.Server(p.client.Conn, p.Config.TLSConfig)

	err := tlsConn.Handshake()
	if err != nil {
		return err
	}
	p.SetClientConn(tlsConn)
	return nil
}

// StartTLSWithServer upgrades the connection with the backend server
func (p *Proxy) StartTLSWithServer() error {
	// When communicating with the server, the proxy is acting as the TLS client.
	tlsConn := tls.Client(p.server.Conn, &tls.Config{InsecureSkipVerify: true})

	err := tlsConn.Handshake()
	if err != nil {
		return err
	}

	return p.SetServerConn(tlsConn)
}

func (p *Proxy) runClientRouter(doneChan chan struct{}) {
	defer func() { doneChan <- struct{}{} }()
	for {
		e, err := p.client.Decoder.NextElement()
		if err != nil {
			// fmt.Println("client decoder error:", err)
			return
		}
		err = p.client.Router.Route(e)
		if err == errStreamOpened {
			_, err = io.Copy(p.server.ReadWriter, p.client.ReadWriter)
		}
		// Let any above errors fall through
		if err != nil {
			// fmt.Println("client router error:", err)
			return
		}
	}
}

func (p *Proxy) runServerRouter(doneChan chan struct{}) {
	defer func() { doneChan <- struct{}{} }()
	for {
		e, err := p.server.Decoder.NextElement()
		if err != nil {
			// fmt.Println("server decoder error:", err)
			return
		}
		err = p.server.Router.Route(e)
		if err == errStreamOpened {
			// When the stream is finally open, expect that the stream features was already parsed and read since reads are buffered, and request the next element as well.
			var e1 xmpp.Element
			e1, err = p.server.Decoder.NextElement()
			if err == nil && e1.Name().Space == xmpp.NSStream && e1.Name().Local == "features" {
				err = p.SendClient(e1.XML())
				if err == nil {
					// Once we are here, the decoder should have nothing left in its buffer and we can just do a byte-level copy of the server conn and write it to the client conn
					_, err = io.Copy(p.client.ReadWriter, p.server.ReadWriter)
				}
			}
		}
		// Let errors from errStreamOpened fall through and be caught here.
		if err != nil {
			// fmt.Println("server router error:", err)
			return
		}
	}
}

func (p *Proxy) setLogName(clientConn net.Conn) error {
	pAddr := prettifyAddress(clientConn.RemoteAddr())
	p.logName = filepath.Join(p.Config.LogPath, pAddr)

	if err := appFS.MkdirAll(p.logName, 0755); err != nil {
		return err
	}

	now := time.Now().Format(p.Config.FileTimeFormat)
	p.logName = filepath.Join(p.logName, now)
	return nil
}

func (p *Proxy) setupClientRouter() {
	// Setup Client Router
	p.client.Router = xmpp.NewRouter()

	// Stream Open Route
	clientStreamOpenRoute := xmpp.NewRoute()
	clientStreamOpenRoute.AddMatcher(xmpp.NameMatcher{Space: xmpp.NSStream, Local: "stream"})
	clientStreamOpenRoute.SetHandler(xmpp.HandlerFunc(func(e xmpp.Element) error {
		if stream, ok := e.(*xmpp.Stream); ok {
			p.client.Stream = stream
			// Check to see if the server has already responded/populated the From attribute. If it has, use that. Otherwise, populate with the configured domain.
			if p.server.Stream != nil && p.server.Stream.From != "" {
				stream.To = p.server.Stream.From
			} else {
				stream.To = p.Config.Domain
			}

			// If SASL has succeeded, there's no need for us to continue parsing the stream XML and want to proceed with just a byte-level copy, so return errStreamOpened
			if p.saslSuccess {
				if err := p.SendServer(stream.XML()); err != nil {
					return err
				}
				return errStreamOpened
			}
			return p.SendServer(stream.XML())
		}
		return fmt.Errorf("expected xmpp.Stream but got something else: %s", e.XML())
	}))
	p.client.Router.AddRoute(clientStreamOpenRoute)

	// StartTLS Route
	p.tlsProceedChan = make(chan struct{})
	clientTLSRoute := xmpp.NewRoute()
	clientTLSRoute.AddMatcher(xmpp.SpaceMatcher(xmpp.NSTLS))
	clientTLSRoute.SetHandler(xmpp.HandlerFunc(func(e xmpp.Element) error {
		if e.Name().Local == "starttls" {
			if err := p.SendServer(e.XML()); err != nil {
				return err
			}
			// after client sends starttls command, the client loop should block until proceed is received.
			<-p.tlsProceedChan
			if err := p.StartTLSWithClient(); err != nil {
				return err
			}
			return nil
		} else {
			return p.SendServer(e.XML())
		}
	}))
	p.client.Router.AddRoute(clientTLSRoute)

	// Default Route
	clientDefaultRoute := xmpp.NewRoute()
	clientDefaultRoute.AddMatcher(xmpp.AllMatcher{})
	clientDefaultRoute.SetHandler(p.server.ForwardHandler)
	p.client.Router.AddRoute(clientDefaultRoute)
}

func (p *Proxy) setupServerRouter() {
	// Setup Server Router
	p.server.Router = xmpp.NewRouter()

	// Stream Open Route
	serverStreamOpenRoute := xmpp.NewRoute()
	serverStreamOpenRoute.AddMatcher(xmpp.NameMatcher{Space: xmpp.NSStream, Local: "stream"})
	serverStreamOpenRoute.SetHandler(xmpp.HandlerFunc(func(e xmpp.Element) error {
		if stream, ok := e.(*xmpp.Stream); ok {
			p.server.Stream = stream
			// If SASL has succeeded, there's no need for us to continue parsing the stream XML and want to proceed with just a byte-level copy, so return errStreamOpened
			if p.saslSuccess {
				if err := p.SendClient(stream.XML()); err != nil {
					return err
				}
				return errStreamOpened
			}
			// Sending XML header is probably unnecessary.
			// if err := p.SendClient(p.server.Decoder.Header); err != nil {
			// 	return err
			// }
			return p.SendClient(stream.XML())
		}
		return fmt.Errorf("expected xmpp.Stream but got something else: %s", e.XML())
	}))
	p.server.Router.AddRoute(serverStreamOpenRoute)

	// StartTLS Route
	serverTLSRoute := xmpp.NewRoute()
	serverTLSRoute.AddMatcher(xmpp.SpaceMatcher(xmpp.NSTLS))
	serverTLSRoute.SetHandler(xmpp.HandlerFunc(func(e xmpp.Element) error {
		if e.Name().Local == "proceed" {
			// https://xmpp.org/rfcs/rfc6120.html#tls-process-initiate-proceed
			if err := p.StartTLSWithServer(); err != nil {
				return err
			}
			if err := p.SendClient(e.XML()); err != nil {
				return err
			}
			// Notify channel that TLS proceed has arrived
			p.tlsProceedChan <- struct{}{}

			return nil
		} else {
			return p.SendClient(e.XML())
		}
	}))
	p.server.Router.AddRoute(serverTLSRoute)

	// SASL Route
	serverSASLRoute := xmpp.NewRoute()
	serverSASLRoute.AddMatcher(xmpp.SpaceMatcher(xmpp.NSSASL))
	serverSASLRoute.SetHandler(xmpp.HandlerFunc(func(e xmpp.Element) error {
		if e.Name().Local == "success" {
			p.saslSuccess = true
		}
		return p.SendClient(e.XML())
	}))
	p.server.Router.AddRoute(serverSASLRoute)

	// Default Route
	serverDefaultRoute := xmpp.NewRoute()
	serverDefaultRoute.AddMatcher(xmpp.AllMatcher{})
	serverDefaultRoute.SetHandler(p.client.ForwardHandler)
	p.server.Router.AddRoute(serverDefaultRoute)
}

func prettifyAddress(addr net.Addr) (pretty string) {
	pretty = addr.String()
	// addr.String() looks like 172.30.127.184:56690
	if s := strings.Split(pretty, ":"); len(s) > 0 {
		// s[0] looks like 172.30.127.184
		pretty = strings.ReplaceAll(s[0], ".", "-")
		// pretty should look like 172-30-127-184
	}
	return
}
