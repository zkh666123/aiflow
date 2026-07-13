package httpapi

import "time"

type Envelope[T any] struct {
	Success   bool      `json:"success"`
	Code      string    `json:"code"`
	Message   string    `json:"message"`
	Data      T         `json:"data"`
	Timestamp time.Time `json:"timestamp"`
}

func SuccessEnvelope[T any](data T, timestamp time.Time) Envelope[T] {
	return Envelope[T]{
		Success:   true,
		Code:      "SUCCESS",
		Message:   "success",
		Data:      data,
		Timestamp: timestamp,
	}
}
