package rpc

import (
	"funpay-parser/internal/models"
	"funpay-parser/internal/runner"
)

type ProgressEvent struct {
	Message string `json:"message"`
}

type RunParserRequest struct {
	Options runner.Options `json:"options"`
}

type RunParserResponse struct {
	Result   runner.Result   `json:"result"`
	Progress []ProgressEvent `json:"progress"`
}

type ClassifyManyRequest struct {
	Listings []models.Listing `json:"listings"`
	Workers  int              `json:"workers"`
}

type ClassifyManyResponse struct {
	Listings []models.Listing `json:"listings"`
}
