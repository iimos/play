package indexdata

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/iimos/play/tr/moexindex"
	"github.com/iimos/play/tr/store"
)

func candle(indexID string, iv uint16) store.IndexCandle {
	return store.IndexCandle{IndexID: indexID, Interval: iv}
}

// Пустой ответ ISS не должен стирать прежние строки: спасаются только те, что
// относятся к пустому ответу, и в исходном порядке партиции.
func TestRescueEmpty(t *testing.T) {
	old := []store.IndexCandle{
		candle("MOEXBMI", 24),
		candle("IMOEX", 24),
		candle("RTSI", 10),
	}
	kept, err := rescueEmpty(context.Background(), "свечи",
		time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC),
		func(context.Context, time.Time) ([]store.IndexCandle, error) { return old, nil },
		func(r store.IndexCandle) bool { return r.IndexID != "MOEXBMI" },
		func(r store.IndexCandle) string { return r.IndexID },
	)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, r := range kept {
		ids = append(ids, r.IndexID)
	}
	if want := []string{"IMOEX", "RTSI"}; !slices.Equal(ids, want) {
		t.Fatalf("kept = %v, want %v", ids, want)
	}
}

// Пустота считается по паре (индекс, интервал): если ISS отдал дневную свечу и
// не отдал 10-минутные, спасаются только 10-минутные строки этого индекса, а
// его дневная свеча берётся из свежего ответа.
func TestRescueEmptyPerInterval(t *testing.T) {
	gaps := map[candleGap]struct{}{
		candleGap{indexID: "IMOEX", interval: 10}: {},
	}
	old := []store.IndexCandle{
		candle("IMOEX", 24), // скачана заново — не спасаем
		candle("IMOEX", 10), // пустой ответ по interval=10 — спасаем
		candle("RTSI", 10),  // индекс не в gap — скачана заново
	}
	kept, err := rescueEmpty(context.Background(), "свечи", time.Now(),
		func(context.Context, time.Time) ([]store.IndexCandle, error) { return old, nil },
		func(r store.IndexCandle) bool {
			_, ok := gaps[candleGap{indexID: r.IndexID, interval: r.Interval}]
			return ok
		},
		func(r store.IndexCandle) string { return r.IndexID },
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != 1 || kept[0].Interval != 10 || kept[0].IndexID != "IMOEX" {
		t.Fatalf("kept = %+v, want only IMOEX interval=10", kept)
	}
}

func TestGapsFor(t *testing.T) {
	active := []moexindex.Info{{ID: "IMOEX"}, {ID: "RTSI"}}
	results := []dayRows{
		// IMOEX: дневная свеча пришла, 10-минутные — нет; веса пришли.
		{candles: []store.IndexCandle{candle("IMOEX", 24)}, emptyIv: []uint16{10}, weights: []store.IndexWeight{{IndexID: "IMOEX"}}},
		// RTSI: пусто по всем интервалам и по весам.
		{emptyIv: []uint16{24, 10}},
	}

	candles, weights := gapsFor(active, results, true)
	for _, want := range []candleGap{{"IMOEX", 10}, {"RTSI", 24}, {"RTSI", 10}} {
		if _, ok := candles[want]; !ok {
			t.Fatalf("missing candle gap %+v in %v", want, candles)
		}
	}
	if _, ok := candles[candleGap{"IMOEX", 24}]; ok {
		t.Fatalf("interval=24 for IMOEX was fetched, must not be a gap")
	}
	if _, ok := weights["RTSI"]; !ok {
		t.Fatalf("weights gap for RTSI must be kept")
	}
	if _, ok := weights["IMOEX"]; ok {
		t.Fatalf("weights for IMOEX were fetched, must not be a gap")
	}

	// Старый день: интрадей не запрашивается, но лежащие в таблице 10-минутные
	// строки терять нельзя.
	candles, _ = gapsFor(active, []dayRows{
		{candles: []store.IndexCandle{candle("IMOEX", 24)}, weights: []store.IndexWeight{{IndexID: "IMOEX"}}},
		{candles: []store.IndexCandle{candle("RTSI", 24)}, weights: []store.IndexWeight{{IndexID: "RTSI"}}},
	}, false)
	for _, id := range []string{"IMOEX", "RTSI"} {
		if _, ok := candles[candleGap{id, intradayInterval}]; !ok {
			t.Fatalf("old day must keep interval=10 for %s", id)
		}
	}
}

// Нет пустых ответов — нет и запроса к таблице.
func TestRescueEmptyNoop(t *testing.T) {
	called := false
	kept, err := rescueEmpty(context.Background(), "свечи", time.Now(),
		func(context.Context, time.Time) ([]store.IndexCandle, error) {
			called = true
			return nil, nil
		},
		nil,
		func(r store.IndexCandle) string { return r.IndexID },
	)
	if err != nil {
		t.Fatal(err)
	}
	if called || len(kept) != 0 {
		t.Fatalf("expected no read: called=%v kept=%v", called, kept)
	}
}

// Пустой ответ был, но прежних строк нет (например веса в выходные): молча
// ничего не возвращаем.
func TestRescueEmptyNothingStored(t *testing.T) {
	kept, err := rescueEmpty(context.Background(), "веса", time.Now(),
		func(context.Context, time.Time) ([]store.IndexWeight, error) { return nil, nil },
		func(store.IndexWeight) bool { return true },
		func(r store.IndexWeight) string { return r.IndexID },
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != 0 {
		t.Fatalf("kept = %+v, want empty", kept)
	}
}

// Ошибка чтения старой партиции не должна проглатываться: иначе день дропнется
// с потерей строк, которых источник не отдал.
func TestRescueEmptyReadError(t *testing.T) {
	wantErr := errors.New("clickhouse is down")
	_, err := rescueEmpty(context.Background(), "свечи", time.Now(),
		func(context.Context, time.Time) ([]store.IndexCandle, error) { return nil, wantErr },
		func(store.IndexCandle) bool { return true },
		func(r store.IndexCandle) string { return r.IndexID },
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want wrap of %v", err, wantErr)
	}
}
