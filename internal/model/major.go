package model

import "time"

// Major represents a school major or field of study.
type Major struct {
	ID        int       `json:"id"`
	Code      string    `json:"code"`
	LongName  string    `json:"long_name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
