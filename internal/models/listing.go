package models

type Listing struct {
	ID                   string   `json:"id"`
	Title                string   `json:"title"`
	Description          string   `json:"description"`
	Price                float64  `json:"price"`
	Currency             string   `json:"currency"`
	Seller               string   `json:"seller"`
	URL                  string   `json:"url"`
	RawHTML              string   `json:"-"`
	IsPlus               *bool    `json:"is_plus"`
	AccountType          *string  `json:"account_type"`
	Confidence           *float64 `json:"confidence"`
	ClassificationReason string   `json:"classification_reason"`
}

func BoolPtr(v bool) *bool        { return &v }
func StringPtr(v string) *string  { return &v }
func FloatPtr(v float64) *float64 { return &v }
