package examples

import (
	"github.com/song940/dhcp-go/dhcp4"
)

type MyServer struct {
}

// HandleDiscover implements dhcp4.ServerMuxHandler.
func (m MyServer) HandleDiscover(discover *dhcp4.Message, rw dhcp4.DiscoverResponseWriter) {
	rw.SendOffer("89.207.132.170")
}

// HandleRenew implements dhcp4.ServerMuxHandler.
func (m MyServer) HandleRenew(renew dhcp4.IGetClientIP, rw dhcp4.RequestResponseWriter) {
	panic("unimplemented")
}

// HandleRequest implements dhcp4.ServerMuxHandler.
func (m MyServer) HandleRequest(request dhcp4.IGetRequestedIP, rw dhcp4.RequestResponseWriter) {
	panic("unimplemented")
}

// HandleDecline implements dhcp4.ServerMuxHandler.
func (m MyServer) HandleDecline(decline dhcp4.IGetRequestedIP, rw dhcp4.RequestResponseWriter) {
	decline.GetRequestedIP()
	rw.SendAck("89.207.132.170")
}

// HandleRelease implements dhcp4.ServerMuxHandler.
func (m MyServer) HandleRelease(release dhcp4.IGetClientIP, rw dhcp4.RequestResponseWriter) {
	panic("unimplemented")
}

func RunServer() {
	server := dhcp4.NewServer()
	my := MyServer{}
	h := dhcp4.NewDefaultServerMux(my)
	server.ListenAndServe(":67", h)
}
