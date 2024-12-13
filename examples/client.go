package examples

import (
	"log"
	"time"

	"github.com/song940/dhcp-go/dhcp4"
)

func RunClient() {
	c, err := dhcp4.NewClient(&dhcp4.ClientConfig{
		Mac:      "aa:bb:cc:dd:ee:ff",
		Hostname: "mypc",
		// Server:   "192.168.2.1",
		// ClientIP: "192.168.2.185",
		Timeout: 30 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()
	offer, err := c.Discover()
	if err != nil {
		log.Fatal(err)
	}
	c.SetServer(offer.ServerIPAddr.String())
	log.Println(offer)
	ack, err := c.Request(offer)
	// ack, err := c.Decline(offer, "no reason")
	// ack, err := c.Renew()
	// ack, err := c.Release()
	// ack, err := c.Inform()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Received:", ack.MessageType(), ack.String())
}
