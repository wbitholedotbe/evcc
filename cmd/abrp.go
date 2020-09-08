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
	Utc string `json:"utc"`
	Soc string `json:"soc"`
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

		if soc, err := v.ChargeState(); err != nil {
			fmt.Printf("State: %v\n", err)
		} else {
			fmt.Printf("State: %.0f%%\n", soc)
			sendSoCState(conf, soc)
		}
	}
}

func sendSoCState(conf config, soc float64) {
	abrpKey := conf.Abrp.Key
	abrpToken := conf.Abrp.Token

	HTTPHelper := util.NewHTTPHelper(util.NewLogger("abrp"))
	/*
		client := &http.Client{
			Timeout: HTTPHelper.Client.Timeout,
		}
	*/

	now := time.Now()
	utc := now.Unix()

	fmt.Printf("Sending at %v\n", now)

	tlmData := &TlmData{
		Utc: strconv.FormatInt(utc, 10),
		Soc: strconv.FormatFloat(soc, 'f', 1, 64),
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
	// reqSend.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var pr ABRPPostResponse
	_, err = HTTPHelper.RequestJSON(reqSend, &pr)

	//	respSend, err := client.Do(reqSend)
	if err != nil {
		fmt.Printf("Error sending SoC: %v\n", err)
		return
	}

	if pr.Status != "ok" {
		fmt.Printf("Error sending SoC: %v\n", pr.Result)
		return
	} else {
		fmt.Printf("Success sending SoC: %v\n", pr.Result)
	}
}
