// Package moexindex предоставляет список индексов MOEX, доступных в ISS
// analytics, и разбор пользовательского выбора индексов.
package moexindex

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/iimos/play/tr/httpjson"
	"github.com/iimos/play/tr/tz"
	"github.com/tidwall/gjson"
)

// listURL возвращает список индексов, по которым ISS публикует аналитику
// (веса бумаг), с датами доступной истории.
const listURL = "https://iss.moex.com/iss/statistics/engines/stock/markets/index/analytics.json"

// securitiesURL — список инструментов рынка индексов с их торговыми атрибутами
// (в т.ч. валютой номинала, CURRENCYID).
const securitiesURL = "https://iss.moex.com/iss/engines/stock/markets/index/securities.json"

// Main — основные индексы; используются как запасной вариант, если список
// всех индексов получить не удалось.
var Main = []string{"IMOEX", "RTSI", "MOEXBMI"}

// Info — сведения об индексе из списка ISS.
type Info struct {
	ID        string
	ShortName string
	From      time.Time
	Till      time.Time
}

// Discover возвращает все индексы, по которым доступна аналитика ISS.
func Discover(ctx context.Context, client *http.Client) ([]Info, error) {
	u := listURL + "?" + url.Values{
		"iss.json": {"extended"},
		"iss.meta": {"off"},
		"iss.only": {"indices"},
	}.Encode()

	body, err := httpjson.GetJSON(ctx, client, u)
	if err != nil {
		return nil, err
	}

	page := gjson.GetBytes(body, "1.indices")
	if !page.IsArray() {
		return nil, fmt.Errorf("unexpected response: %s", httpjson.Truncate(string(body), 200))
	}

	var res []Info
	page.ForEach(func(_, r gjson.Result) bool {
		res = append(res, Info{
			ID:        r.Get("indexid").String(),
			ShortName: r.Get("shortname").String(),
			From:      parseDate(r.Get("from").String()),
			Till:      parseDate(r.Get("till").String()),
		})
		return true
	})
	return res, nil
}

// Currencies возвращает валюту номинала (CURRENCYID) по каждому индексу
// рынка индексов. Индекс может быть номинирован не в рублях (RUBMI, RTS* — USD,
// IMOEXCNY и др. — CNY), что важно учитывать при синтезе.
func Currencies(ctx context.Context, client *http.Client) (map[string]string, error) {
	u := securitiesURL + "?" + url.Values{
		"iss.json": {"extended"},
		"iss.meta": {"off"},
		"iss.only": {"securities"},
	}.Encode()

	body, err := httpjson.GetJSON(ctx, client, u)
	if err != nil {
		return nil, err
	}

	page := gjson.GetBytes(body, "1.securities")
	if !page.IsArray() {
		return nil, fmt.Errorf("unexpected response: %s", httpjson.Truncate(string(body), 200))
	}

	res := map[string]string{}
	page.ForEach(func(_, r gjson.Result) bool {
		id := r.Get("SECID").String()
		if id != "" {
			res[id] = r.Get("CURRENCYID").String()
		}
		return true
	})
	return res, nil
}

// Resolve превращает выбор пользователя в список id индексов:
//   - пусто                     -> все доступные (Discover), при ошибке — Main;
//   - содержит "all"            -> все доступные (Discover);
//   - иначе                     -> нормализованный список.
func Resolve(ctx context.Context, client *http.Client, requested []string) ([]string, error) {
	if len(requested) == 0 {
		ids, err := allIndices(ctx, client)
		if err != nil {
			return append([]string(nil), Main...), nil
		}
		return ids, nil
	}

	for _, id := range requested {
		if strings.EqualFold(strings.TrimSpace(id), "all") {
			return allIndices(ctx, client)
		}
	}

	seen := make(map[string]struct{}, len(requested))
	var res []string
	for _, id := range requested {
		id = strings.ToUpper(strings.TrimSpace(id))
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		res = append(res, id)
	}
	return res, nil
}

// allIndices возвращает id всех индексов, по которым ISS публикует аналитику.
func allIndices(ctx context.Context, client *http.Client) ([]string, error) {
	infos, err := Discover(ctx, client)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(infos))
	for _, in := range infos {
		if in.ID != "" {
			ids = append(ids, in.ID)
		}
	}
	return ids, nil
}

func parseDate(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.ParseInLocation(time.DateOnly, s, tz.MSK)
	if err != nil {
		return time.Time{}
	}
	return t
}
