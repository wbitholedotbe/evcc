package main

import (
	"context"
	"log"
	"time"

	"github.com/andig/evcc/hems/eebus/ship"
	"github.com/grandcat/zeroconf"
)

const (
	zeroconfType   = "_ship._tcp"
	zeroconfDomain = "local."
)

func discoverDNS(results <-chan *zeroconf.ServiceEntry) {
	for entry := range results {
		// log.Printf("%+v", entry)
		ss, err := ship.NewFromDNSEntry(entry)
		if err == nil {
			err = ss.Connect()
			log.Printf("%s: %+v", entry.HostName, ss)
		}

		if err == nil {
			err = ss.Close()
		}

		if err != nil {
			log.Println(err)
		}
	}
}

func main() {
	// Discover all services on the network (e.g. _workstation._tcp)
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Fatalln("Failed to initialize resolver:", err.Error())
	}

	entries := make(chan *zeroconf.ServiceEntry)
	go discoverDNS(entries)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	if err = resolver.Browse(ctx, zeroconfType, zeroconfDomain, entries); err != nil {
		log.Fatalln("Failed to browse:", err.Error())
	}

	<-ctx.Done()
}
