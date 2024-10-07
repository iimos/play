package moexalgo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultAPIURL    = "https://iss.moex.com/iss/"
	DefaultAuthURL   = "https://passport.moex.com/authenticate"
	DefaultPageLimit = 1000
)

// Debug enables logging
var Debug = false

type Session struct {
	http *http.Client
}

type Params struct {
	Username string
	Password string
	Timeout  time.Duration
}

func NewSession(params Params) (*Session, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	if params.Timeout == 0 {
		params.Timeout = 30 * time.Second
	}

	sess := &Session{
		http: &http.Client{
			Jar:     jar,
			Timeout: params.Timeout,
		},
	}
	// если не пустое имя пользователя и пароль = проведем авторизацию
	if params.Username != "" && params.Password != "" {
		err = sess.Auth(params.Username, params.Password)
		if err != nil {
			return nil, err
		}
	}
	return sess, nil
}

func (s *Session) Auth(username, password string) error {
	log("GET " + DefaultAuthURL)
	req, err := http.NewRequest(http.MethodGet, DefaultAuthURL, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(username, password)
	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("moexalgo: auth failed: %s", string(data))
	}
	return nil
}

func Get[T any, PT interface {
	FillerFrom
	*T // T must be such that *T is a FillerFrom
}](ctx context.Context, s *Session, url string, fn func(*T)) error {
	fullURL := DefaultAPIURL + url
	log("GET " + fullURL)
	req, err := http.NewRequest(http.MethodGet, fullURL, nil)
	if err != nil {
		return err
	}
	req.WithContext(ctx)

	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("moexalgo.Get: %s", string(body))
	}

	var r apiResponse
	if err = json.Unmarshal(body, &r); err != nil {
		return fmt.Errorf("moexalgo.Get: %w", err)
	}

	dataSection, ok := r.getData()
	if !ok {
		return fmt.Errorf("moexalgo.Get: failed to find data in the API response: %s; url=%s", string(body), url)
	}

	data, err := transform[T, PT](dataSection)
	if err != nil {
		return fmt.Errorf("moexalgo.Get: %w", err)
	}

	for i := range data {
		fn(&data[i])
	}
	return nil
}

func GetAll[T any, PT interface {
	FillerFrom
	*T // T must be such that *T is a FillerFrom
}](ctx context.Context, s *Session, url string, fn func(*T)) error {
	if strings.Contains(url, "start=") {
		return fmt.Errorf("moexalgo.GetAll: provided URL must not contain 'start' parameter, url=%s", url)
	}

	start := 0
	urlPrefix := url + "&limit=" + strconv.Itoa(DefaultPageLimit) + "&start="
	for {
		prevStart := start
		err := Get[T, PT](ctx, s, urlPrefix+strconv.Itoa(start), func(x *T) {
			fn(x)
			start++
		})
		if err != nil {
			return fmt.Errorf("moexalgo.GetAll: %w", err)
		}
		if prevStart == start {
			return nil
		}
	}
}

type apiResponse struct {
	Data       *APIResponseData `json:"data"`
	Candles    *APIResponseData `json:"candles"`
	MarketData *APIResponseData `json:"marketdata"`
	Securities *APIResponseData `json:"securities"`
	OrderBook  *APIResponseData `json:"orderbook"`
	History    *APIResponseData `json:"history"`
}

func (r *apiResponse) getData() (APIResponseData, bool) {
	// going through all the sections looking for filled one
	if r.Data != nil {
		return *r.Data, true
	}
	if r.Candles != nil {
		return *r.Candles, true
	}
	if r.MarketData != nil {
		return *r.MarketData, true
	}
	if r.Securities != nil {
		return *r.Securities, true
	}
	if r.OrderBook != nil {
		return *r.OrderBook, true
	}
	if r.History != nil {
		return *r.History, true
	}
	return APIResponseData{}, false
}

type APIResponseData struct {
	Columns []string `json:"columns"`
	Data    [][]any  `json:"data"`
}

type FillerFrom interface {
	FillFrom(columns []string, data []any) error
}

func transform[T any, PT interface {
	FillerFrom
	*T
}](data APIResponseData) ([]T, error) {
	ret := make([]T, len(data.Data))
	for i := range data.Data {
		var x PT = &ret[i]
		err := x.FillFrom(data.Columns, data.Data[i])
		if err != nil {
			return nil, err
		}
	}
	return ret, nil
}

func number(i interface{}) float64 {
	f := i.(float64)
	if f == -1 {
		f = 0
	}
	return f
}

func log(msg string, args ...any) {
	if Debug {
		slog.Info("moexalgo: "+msg, args...)
	}
}
