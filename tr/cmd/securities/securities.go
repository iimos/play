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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return store.SecurityInfo{}, false, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return store.SecurityInfo{}, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return store.SecurityInfo{}, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
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

	return store.SecurityInfo{
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
	}, true, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n]) + "..."
}
