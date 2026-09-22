package securities

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/iimos/play/tr/httpjson"
	"github.com/iimos/play/tr/store"
	"github.com/tidwall/gjson"
	"golang.org/x/sync/errgroup"
)

const (
	securitiesURL = "https://iss.moex.com/iss/securities.json"
	bondsListURL  = "https://iss.moex.com/iss/engines/stock/markets/bonds/securities.json"
)

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

	bondSecids, err := listBondSecIDs(ctx, client)
	if err != nil {
		return fmt.Errorf("list bonds: %w", err)
	}
	fmt.Printf("Found %d bonds in MOEX listing\n", len(bondSecids))

	secids = dedupe(append(secids, bondSecids...))

	var (
		mu      sync.Mutex
		infos   []store.SecurityInfo
		skipped int
	)

	gr, gctx := errgroup.WithContext(ctx)
	gr.SetLimit(10)

	for _, secid := range secids {
		secid := secid
		gr.Go(func() error {
			info, ok, err := fetchSecurityInfo(gctx, client, secid)
			if err != nil {
				mu.Lock()
				skipped++
				mu.Unlock()
				fmt.Printf("  %s: error: %v (skipped)\n", secid, err)
				return nil
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

	fmt.Printf("Fetched %d/%d securities (skipped %d)\n", len(infos), len(secids), skipped)

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

	body, err := httpjson.GetJSON(ctx, client, u)
	if err != nil {
		return store.SecurityInfo{}, false, err
	}

	rows := gjson.GetBytes(body, "1.securities")
	if !rows.IsArray() {
		return store.SecurityInfo{}, false, fmt.Errorf("unexpected response: %s", httpjson.Truncate(string(body), 200))
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
		b, err = fetchFuturesInfo(ctx, client, info.SecID, info.PrimaryBoardID)
		if err != nil {
			return store.SecurityInfo{}, false, err
		}
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
	info.ContractName = b.ContractName
	info.LastTradeDate = b.LastTradeDate
	info.LastDelDate = b.LastDelDate

	return info, true, nil
}

// listBondSecIDs возвращает SECID всех облигаций из листинга MOEX.
// Эндпоинт листинга игнорирует параметр start и отдаёт все строки разом,
// поэтому пагинация не применяется.
func listBondSecIDs(ctx context.Context, client *http.Client) ([]string, error) {
	u := bondsListURL + "?" + url.Values{
		"iss.json": {"extended"},
		"iss.meta": {"off"},
		"iss.only": {"securities"},
	}.Encode()

	body, err := httpjson.GetJSON(ctx, client, u)
	if err != nil {
		return nil, err
	}

	rows := gjson.GetBytes(body, "1.securities")
	if !rows.IsArray() {
		return nil, fmt.Errorf("unexpected response: %s", httpjson.Truncate(string(body), 200))
	}

	var secids []string
	rows.ForEach(func(_, r gjson.Result) bool {
		secids = append(secids, r.Get("SECID").String())
		return true
	})
	return secids, nil
}

// dedupe убирает дубликаты из среза строк, сохраняя порядок.
func dedupe(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
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
	case group == "stock_bonds":
		return "stock", "bonds", true
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
	ContractName    string
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

	body, err := httpjson.GetJSON(ctx, client, u)
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

// fetchFuturesInfo получает статические параметры фьючерса: торговые — с борда,
// недостающие (у истёкших контрактов, которых уже нет в борд-листинге) —
// из detail-эндпоинта. CONTRACTNAME есть только в detail-описании, поэтому
// запрашивается всегда.
func fetchFuturesInfo(ctx context.Context, client *http.Client, secid, boardid string) (boardInfo, error) {
	b := fetchBoardInfo(ctx, client, "futures_forts", boardid, secid)
	d, err := fetchFuturesDescription(ctx, client, secid)
	if err != nil {
		return boardInfo{}, err
	}

	b.ContractName = d.ContractName
	if b.LotSize == 0 {
		b.LotSize = d.LotSize
	}
	if b.AssetCode == "" {
		b.AssetCode = d.AssetCode
	}
	if b.FaceUnit == "" {
		b.FaceUnit = d.FaceUnit
	}
	if b.LastTradeDate == nil {
		b.LastTradeDate = d.LastTradeDate
	}
	if b.LastDelDate == nil {
		b.LastDelDate = d.LastDelDate
	}
	return b, nil
}

// fetchFuturesDescription получает статические параметры фьючерса из
// detail-эндпоинта /iss/securities/{secid} (блок description).
func fetchFuturesDescription(ctx context.Context, client *http.Client, secid string) (boardInfo, error) {
	u := "https://iss.moex.com/iss/securities/" + url.PathEscape(secid) + ".json?" + url.Values{
		"iss.json": {"extended"},
		"iss.meta": {"off"},
		"iss.only": {"description"},
	}.Encode()

	body, err := httpjson.GetJSON(ctx, client, u)
	if err != nil {
		return boardInfo{}, err
	}

	var b boardInfo
	gjson.GetBytes(body, "1.description").ForEach(func(_, r gjson.Result) bool {
		switch r.Get("name").String() {
		case "LOTSIZE":
			b.LotSize = r.Get("value").Float()
		case "ASSETCODE":
			b.AssetCode = r.Get("value").String()
		case "FACEUNIT":
			b.FaceUnit = r.Get("value").String()
		case "CONTRACTNAME":
			b.ContractName = r.Get("value").String()
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

	return b, nil
}

func parseISODate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}
