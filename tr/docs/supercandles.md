# Суперсвечи — 5-минутные метрики MOEX (акции, фьючерсы, валюта)

Датасет **SuperCandles** — 5-минутные метрики по инструментам Московской биржи:
сделки, поток заявок и состояние стакана. Для каждого интервала три независимых
набора полей объединяются по ключу `(time, secid)`.

- Источник: MOEX AlgoPack — три эндпоинта на рынок
  - `datashop/algopack/{eq,fo,fx}/tradestats.json` — метрики сделок
  - `datashop/algopack/{eq,fo,fx}/obstats.json` — метрики стакана
  - `datashop/algopack/{eq,fo,fx}/orderstats.json` — метрики заявок
- Периодичность: **5 минут**, публикация — через несколько секунд после закрытия интервала
- Глубина: с **2022-01**
- Ключ: `(secid, time)`, `ORDER BY (secid, time)`, `PARTITION BY Date(time)`

## Таблицы

| Таблица | Рынок | Состав полей |
|---|---|---|
| `tr.super_eq` | акции | tradestats + obstats + orderstats |
| `tr.super_fo` | фьючерсы | tradestats (+ `im`, `oi_*`) + obstats (FO-набор) |
| `tr.super_fx` | валюта | tradestats + obstats (FO-набор) + orderstats |

Полные определения колонок (с комментариями по каждому полю) — в
[`sql/tables.sql`](../sql/tables.sql).

## Группы полей

### tradestats — метрики сделок

OHLC (`pr_open/high/low/close`), волатильность `pr_std`, объём и оборот
(`vol`, `val`), число сделок `trades`, `pr_vwap`, `pr_change`, разбивка
покупки/продажи (`trades_b/_s`, `val_b/_s`, `vol_b/_s`, `pr_vwap_b/_s`) и
дисбаланс `disb`. Поля `sec_pr_*` — цены первой/последней секунды интервала.

### obstats — метрики стакана

- `super_eq`: `spread_bbo`, `spread_lv10`, `spread_1mio`, `levels_b/_s`,
  `imbalance_vol_bbo/_val_bbo`, `imbalance_vol/_val`, `vwap_b/_s`,
  `vwap_b_1mio`/`vwap_s_1mio`.
- `super_fo`/`super_fx`: `mid_price`, `micro_price`, `spread_l1..l10`
  (у `super_fo` — до `l20`), кумулятивная глубина
  `vol_b_l1..`/`vol_s_l1..` (у `super_fo` — до `l20`), `vwap_*_l3..`.

### orderstats — метрики заявок

Выставленные и снятые заявки: `put_*` и `cancel_*`
(`*_orders_b/_s`, `*_val_b/_s`, `*_vol_b/_s`, `*_vwap_b/_s`, агрегаты
`put_orders/put_val/put_vol/cancel_*`). Есть у `super_eq` и `super_fx`.

### Фьючерсные поля (`super_fo`)

`asset_code` (базовый актив, `ASSETCODE`) и открытый интерес
`oi_open/high/low/close`, гарантийное обеспечение `im`.

## Открытая (текущая) пятиминутка

`super_eq/fo/fx` содержат только **закрытые** интервалы. Текущая, ещё не
закрытая свечка (то, что API отдаёт по `latest=1`) хранится **отдельно** — в
таблицах `tr.super_eq_latest` / `tr.super_fo_latest` / `tr.super_fx_latest`
(`ReplacingMergeTree(updated_at)`, одна живая строка на инструмент). Подробнее —
[`supercandles_latest.md`](./supercandles_latest.md).

## Ссылки

- SuperCandles (методология): https://moexalgo.github.io/docs/method/supercandles/
- Синтез суперсвечей индексов: [index_super.md](./index_super.md)
