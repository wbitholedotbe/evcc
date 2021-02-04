package kamereon

import (
	"time"

	"github.com/andig/evcc/api"
)

// Response structure for kamereon api
type Response struct {
	Data data `json:"data"`
}

type data struct {
	Attributes attributes `json:"attributes"`
}

type attributes struct {
	Timestamp          string  `json:"timestamp"`
	ChargingStatus     float32 `json:"chargingStatus"`
	InstantaneousPower int     `json:"instantaneousPower"`
	RangeHvacOff       int     `json:"rangeHvacOff"`    // Nissan
	BatteryAutonomy    int     `json:"batteryAutonomy"` // Renault
	BatteryLevel       int     `json:"batteryLevel"`
	BatteryCapacity    int     `json:"batteryCapacity"` // Nissan
	BatteryTemperature int     `json:"batteryTemperature"`
	PlugStatus         int     `json:"plugStatus"`
	LastUpdateTime     string  `json:"lastUpdateTime"`
	ChargePower        int     `json:"chargePower"`
	RemainingTime      *int    `json:"chargingRemainingTime"`
}

// API is a kamereon API implementation
type API struct {
	apiG func() (interface{}, error)
}

// New returns a kamereon API implementation
func New(apiG func() (interface{}, error)) *API {
	return &API{apiG: apiG}
}

// SoC implements the api.Vehicle interface
func (v *API) SoC() (float64, error) {
	res, err := v.apiG()

	if res, ok := res.(Response); err == nil && ok {
		return float64(res.Data.Attributes.BatteryLevel), nil
	}

	return 0, err
}

// Status implements the Vehicle.Status interface
func (v *API) Status() (api.ChargeStatus, error) {
	status := api.StatusA // disconnected

	res, err := v.apiG()
	if res, ok := res.(Response); err == nil && ok {
		if res.Data.Attributes.PlugStatus > 0 {
			status = api.StatusB
		}
		if res.Data.Attributes.ChargingStatus > 1.0 {
			status = api.StatusC
		}
	}

	return status, err
}

// Range implements the api.VehicleRange interface
func (v *API) Range() (int64, error) {
	res, err := v.apiG()

	if res, ok := res.(Response); err == nil && ok {
		return int64(res.Data.Attributes.RangeHvacOff), nil
	}

	return 0, err
}

// FinishTime implements the api.VehicleFinishTimer interface
func (v *API) FinishTime() (time.Time, error) {
	res, err := v.apiG()

	if res, ok := res.(Response); err == nil && ok {
		timestamp, err := time.Parse(time.RFC3339, res.Data.Attributes.Timestamp)

		if res.Data.Attributes.RemainingTime == nil {
			return time.Time{}, api.ErrNotAvailable
		}

		return timestamp.Add(time.Duration(*res.Data.Attributes.RemainingTime) * time.Minute), err
	}

	return time.Time{}, err
}
