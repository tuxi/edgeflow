package model

import "time"

type ExecutionLog struct {
	Timestamp time.Time `json:"timestamp"`
	Strategy  string    `json:"strategy"`
	Symbol    string    `json:"symbol"`
	Side      string    `json:"side"`
	Price     float64   `json:"price"`
	Note      string    `json:"note,omitempty"`
}
