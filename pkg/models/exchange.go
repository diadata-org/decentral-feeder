package models

type Exchange struct {
	Name        string `json:"Name"`
	Centralized bool   `json:"Centralized"`
	// TO DO: Do we need bridge?
	Bridge     bool   `json:"Bridge"`
	Contract   string `json:"Contract"`
	Blockchain string `json:"Blockchain"`
}
