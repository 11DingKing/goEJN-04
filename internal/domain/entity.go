package domain

import "time"

// EntityType identifies a class of physical asset in the microgrid.
type EntityType string

const (
	EntityTypeBatteryCabin EntityType = "battery_cabin"
	EntityTypePowerStation EntityType = "power_station"
	EntityTypeDieselGen    EntityType = "diesel_generator"
)

// BatteryCabin represents a storage battery cabin (储能电池舱). It tracks the
// single hottest cell temperature, state-of-charge and whether charging is
// currently frozen due to a thermal alarm.
type BatteryCabin struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	CellTempC      float64   `json:"cell_temp_c"`
	SOC            float64   `json:"soc_percent"`
	Charging       bool      `json:"charging"`
	ChargingFrozen bool      `json:"charging_frozen"`
	Alarmed        bool      `json:"alarmed"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// EvaluateTemperature checks the cell temperature against the configured
// threshold. When the temperature exceeds the threshold the cabin is
// immediately alarmed and charging is frozen. The return value indicates
// whether an alarm was raised.
func (bc *BatteryCabin) EvaluateTemperature(thresholdC float64, now time.Time) bool {
	if bc.CellTempC > thresholdC {
		bc.Alarmed = true
		bc.ChargingFrozen = true
		bc.Charging = false
		bc.UpdatedAt = now
		return true
	}
	if !bc.Alarmed {
		bc.UpdatedAt = now
	}
	return false
}

// CanOperateOffGrid reports whether the state-of-charge is at or above the
// minimum required for off-grid (islanded) operation.
func (bc BatteryCabin) CanOperateOffGrid(minSOC float64) bool {
	return bc.SOC >= minSOC
}

// PowerStation represents a wind or solar power station (风电光伏场站).
type PowerStation struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Source    string    `json:"source"` // "wind" or "solar"
	Active    bool      `json:"active"`
	OutputKW  float64   `json:"output_kw"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DieselGenerator represents a diesel emergency generator set
// (柴油应急发电机组) used for black-start and backup supply.
type DieselGenerator struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Running   bool      `json:"running"`
	OutputKW  float64   `json:"output_kw"`
	UpdatedAt time.Time `json:"updated_at"`
}
