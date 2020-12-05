package cmd

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/andig/evcc/detect"
	"github.com/andig/evcc/util"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// detectCmd represents the vehicle command
var detectCmd = &cobra.Command{
	Use:   "detect [host ...] [subnet ...]",
	Short: "Auto-detect compatible hardware",
	Long: `Automatic discovery using detect scans the local network for available devices.
Scanning focuses on devices that are commonly used that are detectable with reasonable efforts.

On successful detection, suggestions for EVCC configuration can be made. The suggestions should simplify
configuring EVCC but are probably not sufficient for fully automatic configuration.`,
	Run: runDetect,
}

func init() {
	rootCmd.AddCommand(detectCmd)
}

// ParseHostIPNet converts host or cidr into a host list
func ParseHostIPNet(arg string) (res []string) {
	if ip := net.ParseIP(arg); ip != nil {
		return []string{ip.String()}
	}

	_, ipnet, err := net.ParseCIDR(arg)

	// simple host
	if err != nil {
		return []string{arg}
	}

	// check subnet size
	if bits, _ := ipnet.Mask.Size(); bits < 24 {
		log.INFO.Println("skipping large subnet:", ipnet)
		return
	}

	ips, err := detect.IPsFromSubnet(arg)
	if err != nil {
		log.FATAL.Fatal(err)
	}

	return ips
}

func display(res []detect.Result) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"IP", "Hostname", "Task", "Details"})
	table.SetAutoMergeCells(true)
	table.SetRowLine(true)

	for _, hit := range res {
		switch hit.ID {
		case detect.TaskPing, detect.TaskTCP80, detect.TaskTCP502:
			continue

		default:
			host := ""
			hosts, err := net.LookupAddr(hit.Host)
			if err == nil && len(hosts) > 0 {
				host = strings.TrimSuffix(hosts[0], ".")
			}

			details := ""
			if hit.Details != nil {
				details = fmt.Sprintf("%+v", hit.Details)
			}

			// fmt.Printf("%-16s %-20s %-16s %s\n", hit.Host, host, hit.ID, details)
			table.Append([]string{hit.Host, host, hit.ID, details})
		}
	}

	fmt.Println("")
	table.Render()

	fmt.Println(`
Please open https://github.com/andig/evcc/issues/new in your browser and copy the
results above into a new issue. Please tell us:

	1. Is the scan result correct?
	2. If not correct: please describe your hardware setup.`)
}

func runDetect(cmd *cobra.Command, args []string) {
	util.LogLevel(viper.GetString("log"), nil)

	fmt.Println(`
Auto detection will now start to scan the network for available devices.
Scanning focuses on devices that are commonly used that are detectable with reasonable efforts.
On successful detection, suggestions for EVCC configuration can be made. The suggestions should simplify
configuring EVCC but are probably not sufficient for fully automatic configuration.`)
	fmt.Println()

	// args
	var hosts []string
	for _, arg := range args {
		hosts = append(hosts, ParseHostIPNet(arg)...)
	}

	// autodetect
	if len(hosts) == 0 {
		ipnets := util.LocalIPs()
		if len(ipnets) == 0 {
			log.FATAL.Fatal("could not find ip")
		}

		myIP := ipnets[0]
		log.INFO.Println("my ip:", myIP.IP)

		ips, err := detect.IPsFromSubnet(myIP.String())
		if err != nil {
			log.FATAL.Fatal(err)
		}

		hosts = append(hosts, "127.0.0.1")
		hosts = append(hosts, ips...)
	}

	// magic happens here
	res := detect.Work(log, 50, hosts)
	display(res)
}
