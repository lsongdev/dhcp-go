package dhcp4

import (
	"net"
)

type ResponseWriter interface {
	WriteResponse(resp *Message) error
}

type responseWriter struct {
	server  *Server
	request *Message
}

func (w *responseWriter) WriteResponse(resp *Message) error {
	resp.OpCode = OpCodeBootReply
	resp.Xid = w.request.Xid
	addr := net.UDPAddr{IP: net.IPv4bcast, Port: 68}
	return w.server.Send(&addr, resp)
}

type Handler interface {
	ServeDHCP(req *Message, rw ResponseWriter)
}

type Server struct {
	conn *net.UDPConn
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) Close() {
	s.conn.Close()
}

func (s *Server) Send(addr net.Addr, message *Message) (err error) {
	_, err = s.conn.WriteTo(message.Bytes(), addr)
	return
}

func (s *Server) ListenAndServe(addr string, handler Handler) (err error) {
	laddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return err
	}
	s.conn, err = net.ListenUDP("udp4", laddr)
	if err != nil {
		return err
	}
	for {
		buf := make([]byte, 2048)
		n, _, err := s.conn.ReadFrom(buf)
		if err != nil {
			return err
		}
		request, err := FromBytes(buf[:n])
		if err != nil {
			return err
		}
		if request.OpCode != OpCodeBootRequest {
			continue
		}
		rw := &responseWriter{
			server:  s,
			request: request,
		}
		go handler.ServeDHCP(request, rw)
	}
}
