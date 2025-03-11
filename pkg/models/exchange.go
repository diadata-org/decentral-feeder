package models

var (
	CEX_SOURCE        = SourceType("CEX")
	DEX_SOURCE        = SourceType("DEX")
	SIMULATION_SOURCE = SourceType("SIM")
)

// @SourceType describes how the source(s) constituting the filter point.
// CEX for a filter point computed from trades originating from CEXes.
// DEX for a filter point computed from trades originating from DEXes.
// SIM for a filter point computed from trades originating from simulation of DEX pools.
type SourceType string

type Exchange struct {
	Name        string `json:"Name"`
	Centralized bool   `json:"Centralized"`
	Simulation  bool   `json:"Simulation"`
	Bridge      bool   `json:"Bridge"`
	Contract    string `json:"Contract"`
	Blockchain  string `json:"Blockchain"`
}

// GetSourceType returns the type of @exchange such as DEX. For CEX an empty SourceType is returned.
func GetSourceType(exchange Exchange) SourceType {
	if exchange.Simulation {
		return SIMULATION_SOURCE
	}
	if !exchange.Simulation && !exchange.Centralized {
		return DEX_SOURCE
	}
	return SourceType("")
}

// GetOracleKey returns a key for an asset (or a pair) that can be used for calling the
// corresponding value in an oracle.
func GetOracleKey(sourceType SourceType, pair Pair) string {
	switch sourceType {
	case SourceType(""):
		return pair.QuoteToken.Symbol + "/USD"
	case SIMULATION_SOURCE:
		return string(SIMULATION_SOURCE) + ":" + pair.QuoteToken.Symbol + "/USD"
	case DEX_SOURCE:
		return string(DEX_SOURCE) + ":" + pair.QuoteToken.Symbol + "/USD"
	default:
		return ""
	}
}

// GetOracleKeySimulation returns a key for an asset (or a pool) that can be used for calling the
// corresponding value in an oracle. It is restricted to values originating from simulated DEX pools.
func GetOracleKeySimulation(pair Pair) string {

	return pair.QuoteToken.Symbol + "/USD"

}
