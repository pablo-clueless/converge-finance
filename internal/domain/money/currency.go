package money

import (
	"database/sql/driver"
	"fmt"
)

// Currency represents an ISO 4217 currency
type Currency struct {
	Code          string
	Name          string
	Symbol        string
	DecimalPlaces int
}

var (
	USD = Currency{Code: "USD", Name: "US Dollar", Symbol: "$", DecimalPlaces: 2}
	NGN = Currency{Code: "NGN", Name: "Nigerian Naira", Symbol: "₦", DecimalPlaces: 2}
	EUR = Currency{Code: "EUR", Name: "Euro", Symbol: "€", DecimalPlaces: 2}
	GBP = Currency{Code: "GBP", Name: "British Pound", Symbol: "£", DecimalPlaces: 2}
	JPY = Currency{Code: "JPY", Name: "Japanese Yen", Symbol: "¥", DecimalPlaces: 0}
	CHF = Currency{Code: "CHF", Name: "Swiss Franc", Symbol: "CHF", DecimalPlaces: 2}
	CAD = Currency{Code: "CAD", Name: "Canadian Dollar", Symbol: "C$", DecimalPlaces: 2}
	AUD = Currency{Code: "AUD", Name: "Australian Dollar", Symbol: "A$", DecimalPlaces: 2}
	CNY = Currency{Code: "CNY", Name: "Chinese Yuan", Symbol: "¥", DecimalPlaces: 2}
	INR = Currency{Code: "INR", Name: "Indian Rupee", Symbol: "₹", DecimalPlaces: 2}
	BRL = Currency{Code: "BRL", Name: "Brazilian Real", Symbol: "R$", DecimalPlaces: 2}
)

var currencyRegistry = map[string]Currency{
	"USD": USD,
	"NGN": NGN,
	"EUR": EUR,
	"GBP": GBP,
	"JPY": JPY,
	"CHF": CHF,
	"CAD": CAD,
	"AUD": AUD,
	"CNY": CNY,
	"INR": INR,
	"BRL": BRL,
}

func GetCurrency(code string) (Currency, error) {
	if c, ok := currencyRegistry[code]; ok {
		return c, nil
	}
	return Currency{}, fmt.Errorf("unknown currency code: %s", code)
}

func MustGetCurrency(code string) Currency {
	c, err := GetCurrency(code)
	if err != nil {
		panic(err)
	}
	return c
}

func RegisterCurrency(c Currency) {
	currencyRegistry[c.Code] = c
}

func ListCurrencies() []Currency {
	currencies := make([]Currency, 0, len(currencyRegistry))
	for _, c := range currencyRegistry {
		currencies = append(currencies, c)
	}
	return currencies
}

func (c Currency) IsValid() bool {
	_, exists := currencyRegistry[c.Code]
	return exists
}

func (c Currency) IsZero() bool {
	return c.Code == ""
}

func (c Currency) Equals(other Currency) bool {
	return c.Code == other.Code
}

func (c Currency) String() string {
	return c.Code
}

func (c Currency) Value() (driver.Value, error) {
	if c.IsZero() {
		return nil, nil
	}
	return c.Code, nil
}

func (c *Currency) Scan(value any) error {
	if value == nil {
		*c = Currency{}
		return nil
	}

	var code string
	switch v := value.(type) {
	case string:
		code = v
	case []byte:
		code = string(v)
	default:
		return fmt.Errorf("cannot scan %T into Currency", value)
	}

	currency, err := GetCurrency(code)
	if err != nil {
		*c = Currency{Code: code, DecimalPlaces: 2}
		return nil
	}
	*c = currency
	return nil
}

func (c Currency) MarshalJSON() ([]byte, error) {
	if c.IsZero() {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf(`"%s"`, c.Code)), nil
}

func (c *Currency) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == `""` {
		*c = Currency{}
		return nil
	}
	code := string(data)
	if len(code) >= 2 && code[0] == '"' && code[len(code)-1] == '"' {
		code = code[1 : len(code)-1]
	}

	currency, err := GetCurrency(code)
	if err != nil {
		*c = Currency{Code: code, DecimalPlaces: 2}
		return nil
	}
	*c = currency
	return nil
}
