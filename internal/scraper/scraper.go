package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"trading-bot/internal/database"
	"trading-bot/internal/events"
	"trading-bot/shared/eventdef"

	"github.com/shopspring/decimal"
)

const binanceFuturesKlineURL = "https://testnet.binancefuture.com/fapi/v1/klines"

// Scraper is responsible for scraping historical kline data from Binance.
type Scraper struct {
	publisher events.Publisher
	repo      database.Repository
	client    *http.Client
	url       string
}

func New(publisher events.Publisher, repo database.Repository) *Scraper {
	return &Scraper{
		publisher: publisher,
		repo:      repo,
		client:    &http.Client{Timeout: 30 * time.Second},
		url:       binanceFuturesKlineURL,
	}
}

func newWithClient(publisher events.Publisher, repo database.Repository, client *http.Client, url string) *Scraper {
	return &Scraper{
		publisher: publisher,
		repo:      repo,
		client:    client,
		url:       url,
	}
}

type klineChunk struct {
	Symbol   string
	Interval string
	StartMs  int64
	EndMs    int64
}

func (s *Scraper) ScrapeSymbols(ctx context.Context, symbols []string, interval string, hours int) (int, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	totalSaved := 0
	var firstErr error

	for _, symbol := range symbols {
		wg.Add(1)
		go func(sym string) {
			defer wg.Done()
			count, err := s.scrapeSymbol(ctx, sym, interval, hours)
			mu.Lock()
			totalSaved += count
			if err != nil && firstErr == nil {
				firstErr = err
			}
			mu.Unlock()
		}(symbol)
	}

	wg.Wait()
	return totalSaved, firstErr
}

func (s *Scraper) scrapeSymbol(ctx context.Context, symbol, interval string, hours int) (int, error) {
	endMs := time.Now().UnixMilli()
	startMs := endMs - int64(hours)*60*60*1000

	if startMs >= endMs {
		log.Printf("[%s] Already up to date", symbol)
		return 0, nil
	}

	var chunks []klineChunk
	chunkDuration := int64(500 * 60 * 1000) // 500 candles * 1m = ~8.3 hours
	for cs := startMs; cs < endMs; cs += chunkDuration {
		ce := cs + chunkDuration
		if ce > endMs {
			ce = endMs
		}
		chunks = append(chunks, klineChunk{Symbol: symbol, Interval: interval, StartMs: cs, EndMs: ce})
	}

	log.Printf("[%s] Scraping %d chunks (%d hours of %s data)", symbol, len(chunks), hours, interval)

	var wg sync.WaitGroup
	sem := make(chan struct{}, 3) // max 3 concurrent requests per symbol
	var mu sync.Mutex
	totalSaved := 0
	var firstErr error

	for i, chunk := range chunks {
		select {
		case <-ctx.Done():
			return totalSaved, ctx.Err()
		default:
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, c klineChunk) {
			defer wg.Done()
			defer func() { <-sem }()

			klines, err := s.fetchKlines(ctx, c)
			if err != nil {
				log.Printf("[%s] chunk %d/%d failed: %v", c.Symbol, idx+1, len(chunks), err)
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}

			if len(klines) > 0 {
				var dbKlines []database.Kline
				for _, kline := range klines {
					if s.publisher != nil {
						s.publisher.Publish(context.Background(), "klines."+c.Symbol, eventdef.NewEvent("kline.new", "scraper", 1, kline))
					}
					
					dbKlines = append(dbKlines, database.Kline{
						Symbol:    kline.Symbol,
						Interval:  kline.Interval,
						OpenTime:  time.UnixMilli(kline.OpenTime),
						CloseTime: time.UnixMilli(kline.CloseTime),
						Open:      decimal.NewFromFloat(kline.Open),
						High:      decimal.NewFromFloat(kline.High),
						Low:       decimal.NewFromFloat(kline.Low),
						Close:     decimal.NewFromFloat(kline.Close),
						Volume:    decimal.NewFromFloat(kline.Volume),
					})
				}

				if s.repo != nil && len(dbKlines) > 0 {
					err := s.repo.SaveKlines(dbKlines)
					if err != nil {
						log.Printf("[%s] failed to save klines to DB: %v", c.Symbol, err)
					}
				}

				mu.Lock()
				totalSaved += len(klines)
				mu.Unlock()
				log.Printf("[%s] chunk %d/%d: processed %d klines", c.Symbol, idx+1, len(chunks), len(klines))
			}
		}(i, chunk)
	}

	wg.Wait()
	return totalSaved, nil
}

type Kline struct {
	Symbol    string  `json:"symbol"`
	Interval  string  `json:"interval"`
	OpenTime  int64   `json:"open_time"`
	CloseTime int64   `json:"close_time"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    float64 `json:"volume"`
}

func (s *Scraper) fetchKlines(ctx context.Context, chunk klineChunk) ([]Kline, error) {
	url := fmt.Sprintf("%s?symbol=%s&interval=%s&startTime=%d&endTime=%d&limit=1500",
		s.url, chunk.Symbol, chunk.Interval, chunk.StartMs, chunk.EndMs)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("binance API returned %d: %s", resp.StatusCode, string(body))
	}

	var raw [][]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var klines []Kline
	for _, r := range raw {
		if len(r) < 11 {
			continue
		}

		openTime, _ := strconv.ParseInt(unquote(r[0]), 10, 64)
		closeTime, _ := strconv.ParseInt(unquote(r[6]), 10, 64)
		open, _ := strconv.ParseFloat(unquote(r[1]), 64)
		high, _ := strconv.ParseFloat(unquote(r[2]), 64)
		low, _ := strconv.ParseFloat(unquote(r[3]), 64)
		close_, _ := strconv.ParseFloat(unquote(r[4]), 64)
		volume, _ := strconv.ParseFloat(unquote(r[5]), 64)

		klines = append(klines, Kline{
			Symbol:    chunk.Symbol,
			Interval:  chunk.Interval,
			OpenTime:  openTime,
			CloseTime: closeTime,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close_,
			Volume:    volume,
		})
	}

	return klines, nil
}

func unquote(raw json.RawMessage) string {
	s := string(raw)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
