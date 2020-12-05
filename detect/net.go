package detect

import (
	"errors"
	"fmt"

	"github.com/andig/evcc/util"
	"github.com/korylprince/ipnetgen"
)

// Hosts returns the list of hosts to scan
func Hosts(all bool) ([]string, error) {
	ipnets := util.LocalIPs()
	if len(ipnets) == 0 {
		return nil, errors.New("could not find ip")
	}

	if !all {
		ipnets = ipnets[:1]
	}

	var hosts []string
	for _, ipnet := range ipnets {
		ips, err := IPsFromSubnet(ipnet.String())
		if err != nil {
			return nil, err
		}

		hosts = append(hosts, ips...)
	}

	return hosts, nil
}

// IPsFromSubnet creates a list of ip addresses for given subnet
func IPsFromSubnet(arg string) ([]string, error) {
	gen, err := ipnetgen.New(arg)
	if err != nil {
		return nil, fmt.Errorf("could not generate IPs for subnet: %w", err)
	}

	var ips []string
	for ip := gen.Next(); ip != nil; ip = gen.Next() {
		ips = append(ips, ip.String())
	}

	return ips, nil
}
