package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/andig/evcc/api"
	"github.com/andig/evcc/server"
	"github.com/andig/evcc/util"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	abrpSend = "https://api.iternio.com/1/tlm/send"
)

type ABRPPostResponse struct {
	Status string `json:"status"`
	Result string `json:"result"`
}

type TlmData struct {
	Utc        string `json:"utc"`
	Soc        string `json:"soc"`
	Ischarging bool   `json:"is_charging"`
}

// abrpCmd represents the ABRP command
var abrpCmd = &cobra.Command{
	Use:   "abrp [vehicle name]",
	Short: "Query configured vehicles",
	Run:   runAbrp,
}

func init() {
	rootCmd.AddCommand(abrpCmd)
}

func runAbrp(cmd *cobra.Command, args []string) {
	util.LogLevel(viper.GetString("log"), viper.GetStringMapString("levels"))
	log.INFO.Printf("evcc %s (%s)", server.Version, server.Commit)

	// load config
	conf := loadConfigFile(cfgFile)

	cp := &ConfigProvider{}
	cp.configureVehicles(conf)

	vehicles := cp.vehicles
	if len(args) == 1 {
		arg := args[0]
		vehicles = map[string]api.Vehicle{arg: cp.Vehicle(arg)}
	}

	for name, v := range vehicles {
		if len(vehicles) != 1 {
			fmt.Println(name)
		}

		soc, err := v.ChargeState()
		if err != nil {
			fmt.Printf("ChargeState: %v\n", err)
			return
		} else {
			fmt.Printf("State: %.0f%%\n", soc)
		}

		isCharging, err := v.ChargingState()
		if err != nil {
			fmt.Printf("ChargingState: %v\n", err)
			return
		} else {
			fmt.Printf("Charging: %t\n", isCharging)
		}

		sendState(conf, soc, isCharging)

	}
}

func sendState(conf config, soc float64, isCharging bool) {
	abrpKey := conf.Abrp.Key
	abrpToken := conf.Abrp.Token

	HTTPHelper := util.NewHTTPHelper(util.NewLogger("abrp"))

	now := time.Now()
	utc := now.Unix()

	fmt.Printf("Sending at %v\n", now)

	tlmData := &TlmData{
		Utc:        strconv.FormatInt(utc, 10),
		Soc:        strconv.FormatFloat(soc, 'f', 1, 64),
		Ischarging: isCharging,
	}

	b, err := json.Marshal(tlmData)
	if err != nil {
		fmt.Printf("Error creating JSON: %v\n", err)
	}

	sendData := url.Values{
		"tlm":   []string{string(b)},
		"token": []string{abrpToken},
	}

	reqSend, err := http.NewRequest(http.MethodPost, abrpSend, nil)
	if err != nil {
		fmt.Printf("Error sending SoC: %v\n", err)
		return
	}

	reqSend.URL.RawQuery = sendData.Encode()

	reqSend.Header.Set("Authorization", fmt.Sprintf("APIKEY %s", abrpKey))

	var pr ABRPPostResponse
	_, err = HTTPHelper.RequestJSON(reqSend, &pr)

	if err != nil {
		fmt.Printf("Error sending data: %v\n", err)
		return
	}

	if pr.Status != "ok" {
		fmt.Printf("Error sending data: %v\n", pr.Result)
		return
	} else {
		fmt.Printf("Success sending data: %v\n", pr.Result)
	}
}
