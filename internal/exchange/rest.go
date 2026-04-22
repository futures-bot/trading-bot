package exchange

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"

	"github.com/adshao/go-binance/v2/common"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/shopspring/decimal"
)

// RestClient handles the communication with the Binance REST API.
type RestClient struct {
	client            *futures.Client
	PricePrecision    int32
	QuantityPrecision int32
	StepSize          decimal.Decimal
}

// NewRestClient creates a new RestClient.
func NewRestClient() *RestClient {
	apiKey := os.Getenv("BINANCE_API_KEY")
	secretKey := os.Getenv("BINANCE_SECRET_KEY")

	client := futures.NewClient(apiKey, secretKey)
	client.BaseURL = "https://testnet.binancefuture.com"
	return &RestClient{client: client}
}

// GetExchangeInfo retrieves the exchange info for the given symbol.
func (c *RestClient) GetExchangeInfo(ctx context.Context, symbol string) error {
	info, err := c.client.NewExchangeInfoService().Do(ctx)
	if err != nil {
		return err
	}

	for _, s := range info.Symbols {
		if s.Symbol == symbol {
			log.Printf("DEBUG: Parsing filters for %s", symbol)
			c.PricePrecision = int32(s.PricePrecision)
			c.QuantityPrecision = int32(s.QuantityPrecision)

			var stepSizeStr string
			if lotSizeFilter := s.LotSizeFilter(); lotSizeFilter != nil {
				stepSizeStr = regexp.MustCompile(`[0-9]+(\.[0-9]+)?`).FindString(lotSizeFilter.StepSize)
			}

			if stepSizeStr == "" || stepSizeStr == "0" {
				if symbol == "SOLUSDT" {
					stepSizeStr = "0.01"
					log.Printf("WARNING: StepSize was empty for SOLUSDT. Hard-coding to 0.01 for safety.")
				} else {
					stepSizeStr = "0.1"
					log.Printf("WARNING: StepSize was empty. Hard-coding to 0.1 for safety.")
				}
			}

			c.StepSize, err = decimal.NewFromString(stepSizeStr)
			if err != nil {
				return fmt.Errorf("error parsing step size '%s': %w", stepSizeStr, err)
			}

			log.Printf("Set precision for %s: Price=%d, Quantity=%d, StepSize=%s", symbol, c.PricePrecision, c.QuantityPrecision, c.StepSize.String())
			return nil
		}
	}

	return nil
}

// VerifyCredentialsAndGetBalance checks if the API keys are valid and retrieves the USDT balance.
func (c *RestClient) VerifyCredentialsAndGetBalance(ctx context.Context) (*futures.Balance, error) {
	balances, err := c.client.NewGetBalanceService().Do(ctx)
	if err != nil {
		if apiErr, ok := err.(*common.APIError); ok && apiErr.Code == -2015 {
			log.Fatal("CRITICAL: Key/URL Mismatch. Ensure you are using Futures Testnet keys, NOT Spot Testnet keys.")
		} else {
			log.Fatalf("Failed to verify credentials: %v", err)
		}
	}
	log.Printf("Full balance response: %+v\n", balances)

	for _, b := range balances {
		if b.Asset == "USDT" {
			log.Printf("USDT Balance: %s", b.Balance)
			return b, nil
		}
	}

	return nil, errors.New("USDT balance not found")
}

// PlaceOrder places a new order on the exchange.
func (c *RestClient) PlaceOrder(ctx context.Context, symbol string, side futures.SideType, quantity string) (*futures.CreateOrderResponse, error) {
	log.Printf("RAW REQUEST QTY: %s", quantity)
	order, err := c.client.NewCreateOrderService().Symbol(symbol).
		Side(side).
		Type(futures.OrderTypeMarket).
		Quantity(quantity).
		Do(ctx)
	if err != nil {
		return nil, err
	}
	return order, nil
}

// SetLeverage sets the leverage for the given symbol.
func (c *RestClient) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	_, err := c.client.NewChangeLeverageService().Symbol(symbol).Leverage(leverage).Do(ctx)
	return err
}

// CancelAllOpenOrders cancels all open orders for the given symbol.
func (c *RestClient) CancelAllOpenOrders(ctx context.Context, symbol string) error {
	return c.client.NewCancelAllOpenOrdersService().Symbol(symbol).Do(ctx)
}

// GetAccountTradeList retrieves the trade list for a given order.
func (c *RestClient) GetAccountTradeList(ctx context.Context, symbol string, orderID int64) ([]*futures.AccountTrade, error) {
	trades, err := c.client.NewListAccountTradeService().Symbol(symbol).OrderID(orderID).Do(ctx)
	if err != nil {
		return nil, err
	}
	return trades, nil
}
