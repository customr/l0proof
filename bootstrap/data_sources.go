package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"
)

type MoexPriceSource struct {
	Date     string
	Interval int
	Ticker   string
	client   *http.Client
}

func NewMoexPriceSource(date string, interval int, ticker string) *MoexPriceSource {
	return &MoexPriceSource{
		Date:     date,
		Interval: interval,
		Ticker:   ticker,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type moexResponse struct {
	Candles struct {
		Columns []string        `json:"columns"`
		Data    [][]interface{} `json:"data"`
	} `json:"candles"`
}

func (s *MoexPriceSource) FetchPrice(ctx context.Context) (float64, error) {
	url := fmt.Sprintf(
		"https://iss.moex.com/iss/engines/stock/markets/shares/securities/%s/candles.json",
		s.Ticker,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}
	q := req.URL.Query()
	q.Add("from", s.Date)
	q.Add("till", s.Date)
	q.Add("interval", fmt.Sprintf("%d", s.Interval))
	req.URL.RawQuery = q.Encode()

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read response body: %w", err)
	}

	var data moexResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return 0, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	if len(data.Candles.Data) == 0 {
		return 0, fmt.Errorf("empty MOEX response")
	}

	// Find indices of needed columns
	var highIdx, lowIdx, closeIdx int = -1, -1, -1
	for i, col := range data.Candles.Columns {
		switch col {
		case "high":
			highIdx = i
		case "low":
			lowIdx = i
		case "close":
			closeIdx = i
		}
	}

	if highIdx == -1 || lowIdx == -1 || closeIdx == -1 {
		return 0, fmt.Errorf("required columns not found in response")
	}

	// Get last candle
	lastCandle := data.Candles.Data[len(data.Candles.Data)-1]

	high, ok := lastCandle[highIdx].(float64)
	if !ok {
		return 0, fmt.Errorf("invalid high price format")
	}

	low, ok := lastCandle[lowIdx].(float64)
	if !ok {
		return 0, fmt.Errorf("invalid low price format")
	}

	closePrice, ok := lastCandle[closeIdx].(float64)
	if !ok {
		return 0, fmt.Errorf("invalid close price format")
	}

	typicalPrice := (high + low + closePrice) / 3
	return typicalPrice, nil
}

type MockPriceSource struct {
	BasePrice float64
	Variation float64
}

func NewMockPriceSource(basePrice float64, variation float64) *MockPriceSource {
	return &MockPriceSource{
		BasePrice: basePrice,
		Variation: variation,
	}
}

func (s *MockPriceSource) FetchPrice(ctx context.Context) (float64, error) {
	variation := (rand.Float64()*2 - 1) * s.Variation
	return s.BasePrice * (1 + variation), nil
}

func CreatePriceSources(ticker string) []PriceSource {
	today := time.Now().UTC().AddDate(0, 0, -2).Format("2006-01-02")

	sources := []PriceSource{
		NewMoexPriceSource(today, 10, ticker),
	}

	var basePrice float64
	switch ticker {
	case "SBER":
		basePrice = 300
	default:
		basePrice = 100.0
	}

	sources = append(sources, NewMockPriceSource(basePrice, 0.01))

	return sources
}
