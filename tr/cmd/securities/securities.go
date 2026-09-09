package securities

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/iimos/play/tr/store"
	"github.com/tidwall/gjson"
	"golang.org/x/sync/errgroup"
)

const securitiesURL = "https://iss.moex.com/iss/securities.json"

// Load получает справочную информацию (название, эмитент и т.п.) по всем тикерам,
// встречающимся в таблицах данных, и сохраняет её в таблицу security_info.
func Load(ctx context.Context) error {
	storage, err := store.New()
	if err != nil {
		return err
	}
	defer storage.Close()

	secids, err := storage.DistinctSecIDs(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("Found %d distinct secids in data tables\n", len(secids))

	client := &http.Client{Timeout: 30 * time.Second}

	var (
		mu    sync.Mutex
		infos []store.SecurityInfo
	)

	gr, gctx := errgroup.WithContext(ctx)
	gr.SetLimit(10)

	for _, secid := range secids {
		secid := secid
		gr.Go(func() error {
			info, ok, err := fetchSecurityInfo(gctx, client, secid)
			if err != nil {
				return fmt.Errorf("%s: %w", secid, err)
			}
			if !ok {
				fmt.Printf("  %s: not found\n", secid)
				return nil
			}
			mu.Lock()
			infos = append(infos, info)
			mu.Unlock()
			return nil
		})
	}

	if err := gr.Wait(); err != nil {
		return err
	}

	fmt.Printf("Fetched %d/%d securities\n", len(infos), len(secids))

	if err := storage.StoreSecurityInfo(ctx, infos); err != nil {
		return fmt.Errorf("store security info: %w", err)
	}

	fmt.Printf("Stored %d securities into security_info\n", len(infos))
	return nil
}

func fetchSecurityInfo(ctx context.Context, client *http.Client, secid string) (store.SecurityInfo, bool, error) {
	u := securitiesURL + "?" + url.Values{
		"q":        {secid},
		"iss.json": {"extended"},
		"iss.meta": {"off"},
		"iss.only": {"securities"},
	}.Encode()

	body, err := getJSON(ctx, client, u)
	if err != nil {
		return store.SecurityInfo{}, false, err
	}

	rows := gjson.GetBytes(body, "1.securities")
	if !rows.IsArray() {
		return store.SecurityInfo{}, false, fmt.Errorf("unexpected response: %s", truncate(string(body), 200))
	}

	var found gjson.Result
	rows.ForEach(func(_, r gjson.Result) bool {
		if r.Get("secid").String() == secid {
			found = r
			return false
		}
		return true
	})

	if !found.Exists() {
		return store.SecurityInfo{}, false, nil
	}

	info := store.SecurityInfo{
		SecID:              found.Get("secid").String(),
		ShortName:          found.Get("shortname").String(),
		Name:               found.Get("name").String(),
		ISIN:               found.Get("isin").String(),
		RegNumber:          found.Get("regnumber").String(),
		IsTraded:           uint8(found.Get("is_traded").Uint()),
		EmitentID:          found.Get("emitent_id").String(),
		EmitentTitle:       found.Get("emitent_title").String(),
		EmitentINN:         found.Get("emitent_inn").String(),
		EmitentOKPO:        found.Get("emitent_okpo").String(),
		Type:               found.Get("type").String(),
		Group:              found.Get("group").String(),
		PrimaryBoardID:     found.Get("primary_boardid").String(),
		MarketpriceBoardID: found.Get("marketprice_boardid").String(),
	}

	var b boardInfo
	if info.Group == "futures_forts" {
		b = fetchFuturesInfo(ctx, client, info.SecID, info.PrimaryBoardID)
	} else {
		b = fetchBoardInfo(ctx, client, info.Group, info.PrimaryBoardID, info.SecID)
	}
	info.LotSize = b.LotSize
	info.TradingCurrency = b.TradingCurrency
	info.Decimals = b.Decimals
	info.MinStep = b.MinStep
	info.FaceValue = b.FaceValue
	info.FaceUnit = b.FaceUnit
	info.AssetCode = b.AssetCode
	info.LastTradeDate = b.LastTradeDate
	info.LastDelDate = b.LastDelDate

	return info, true, nil
}

// engineMarketByGroup определяет engine/market ISS по группе инструмента.
func engineMarketByGroup(group string) (engine, market string, ok bool) {
	switch {
	case group == "futures_forts":
		return "futures", "forts", true
	case group == "currency_selt" || group == "currency_metal":
		return "currency", "selt", true
	case group == "stock_shares" || group == "stock_ppif" || group == "stock_dr":
		return "stock", "shares", true
	}
	return "", "", false
}

// boardInfo — статические торговые параметры инструмента.
type boardInfo struct {
	LotSize         float64
	TradingCurrency string
	Decimals        uint8
	MinStep         float64
	FaceValue       float64
	FaceUnit        string
	AssetCode       string
	LastTradeDate   *time.Time
	LastDelDate     *time.Time
}

// fetchBoardInfo получает статические торговые параметры с борда ISS.
func fetchBoardInfo(ctx context.Context, client *http.Client, group, boardid, secid string) boardInfo {
	engine, market, ok := engineMarketByGroup(group)
	if !ok || boardid == "" {
		return boardInfo{}
	}

	u := "https://iss.moex.com/iss/engines/" + url.PathEscape(engine) +
		"/markets/" + url.PathEscape(market) +
		"/boards/" + url.PathEscape(boardid) +
		"/securities/" + url.PathEscape(secid) + ".json?" + url.Values{
		"iss.json": {"extended"},
		"iss.meta": {"off"},
		"iss.only": {"securities"},
	}.Encode()

	body, err := getJSON(ctx, client, u)
	if err != nil {
		return boardInfo{}
	}

	rows := gjson.GetBytes(body, "1.securities")
	if !rows.IsArray() || len(rows.Array()) == 0 {
		return boardInfo{}
	}
	row := rows.Array()[0]

	var b boardInfo
	if v := row.Get("LOTSIZE"); v.Exists() {
		b.LotSize = v.Float()
	} else if v := row.Get("LOTVOLUME"); v.Exists() {
		b.LotSize = v.Float()
	}
	b.TradingCurrency = row.Get("CURRENCYID").String()
	b.Decimals = uint8(row.Get("DECIMALS").Uint())
	b.MinStep = row.Get("MINSTEP").Float()
	b.FaceValue = row.Get("FACEVALUE").Float()
	b.FaceUnit = row.Get("FACEUNIT").String()
	b.AssetCode = row.Get("ASSETCODE").String()
	if t, err := parseISODate(row.Get("LASTTRADEDATE").String()); err == nil {
		b.LastTradeDate = &t
	}
	if t, err := parseISODate(row.Get("LASTDELDATE").String()); err == nil {
		b.LastDelDate = &t
	}

	return b
}

// fetchFuturesInfo получает статические параметры фьючерса. Для торгуемых
// контрактов берёт полные данные с борда (включая decimals/minstep), для
// истёкших (которых уже нет в борд-листинге) — из detail-эндпоинта.
func fetchFuturesInfo(ctx context.Context, client *http.Client, secid, boardid string) boardInfo {
	b := fetchBoardInfo(ctx, client, "futures_forts", boardid, secid)
	if b.LotSize != 0 || b.AssetCode != "" || b.LastTradeDate != nil {
		return b
	}

	u := "https://iss.moex.com/iss/securities/" + url.PathEscape(secid) + ".json?" + url.Values{
		"iss.json": {"extended"},
		"iss.meta": {"off"},
		"iss.only": {"description"},
	}.Encode()

	body, err := getJSON(ctx, client, u)
	if err != nil {
		return boardInfo{}
	}

	gjson.GetBytes(body, "1.description").ForEach(func(_, r gjson.Result) bool {
		switch r.Get("name").String() {
		case "LOTSIZE":
			b.LotSize = r.Get("value").Float()
		case "ASSETCODE":
			b.AssetCode = r.Get("value").String()
		case "FACEUNIT":
			b.FaceUnit = r.Get("value").String()
		case "LSTTRADE":
			if t, err := parseISODate(r.Get("value").String()); err == nil {
				b.LastTradeDate = &t
			}
		case "LSTDELDATE":
			if t, err := parseISODate(r.Get("value").String()); err == nil {
				b.LastDelDate = &t
			}
		}
		return true
	})

	return b
}

// getJSON выполняет GET-запрос и возвращает тело ответа. Транзиентные ошибки
// (сеть, 5xx, 429) ретраятся с экспоненциальной паузой, остальные 4xx — нет.
func getJSON(ctx context.Context, client *http.Client, u string) ([]byte, error) {
	const maxAttempts = 3
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
		if err != nil {
			return nil, err
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode == http.StatusOK {
			return body, nil
		}

		lastErr = fmt.Errorf("%s: http %d", u, resp.StatusCode)
		if !retriableStatus(resp.StatusCode) {
			return nil, lastErr
		}
	}
	return nil, lastErr
}

// retriableStatus возвращает true для транзиентных HTTP-статусов.
func retriableStatus(code int) bool {
	return code >= 500 || code == http.StatusTooManyRequests || code == http.StatusRequestTimeout
}

func parseISODate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n]) + "..."
}
