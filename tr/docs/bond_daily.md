# bond_daily — дневные свечи облигаций

Дневные свечи (OHLCV) по облигациям Московской биржи вместе с облигационными
атрибутами (НКД, доходность, дюрация, купон, номинал, погашение).

- Источник: MOEX ISS **history** — `https://iss.moex.com/iss/history/engines/stock/markets/bonds/boards/{board}/securities.json?date=YYYY-MM-DD`
- Периодичность: **ежедневный срез** (одна строка на инструмент/борд за торговый день)
- Охват: все облигации, торговавшиеся в этот день на основных бордах T+
- Команда загрузки: `go run main.go load-bond-daily` (входит и в `load`)
- Свечи дневные — часовых/минутных по облигациям нет (ISS отдаёт интрадей-свечи только по одному инструменту за запрос; для «всех за дату» есть только history)

## Таблица `tr.bond_daily`

```sql
CREATE TABLE tr.bond_daily (
    time          DateTime CODEC(DoubleDelta(1), LZ4), -- tradedate (день, 00:00 MSK)
    secid         LowCardinality(String), -- код инструмента (тикер)
    boardid       LowCardinality(String), -- режим торгов: TQCB / TQOB / TQDB
    open          Float64, -- цена открытия (чистая, без НКД, % от номинала)
    high          Float64,
    low           Float64,
    close         Float64, -- цена закрытия (чистая, без НКД, % от номинала)
    value         Float64, -- оборот в рублях
    volume        UInt64,  -- объём в лотах/штуках
    numtrades     UInt32,  -- количество сделок
    accint        Float64, -- накопленный купонный доход (НКД) на закрытие, в валюте номинала
    yieldclose    Float64, -- доходность к погашению по цене закрытия, %
    yieldatwap    Float64, -- доходность по средневзвешенной цене, %
    waprice       Float64, -- средневзвешенная цена
    duration      Nullable(Float64), -- дюрация, дней
    couponpercent Float64, -- ставка купона, %
    couponvalue   Float64, -- сумма купона на облигацию
    facevalue     Float64, -- номинал
    faceunit      LowCardinality(String), -- валюта номинала
    currencyid    LowCardinality(String), -- валюта расчётов
    matdate       Nullable(Date), -- дата погашения
    bondtype      LowCardinality(String), -- тип облигации («Фикс с известным купоном», «Флоатер», …)
    bondsubtype   LowCardinality(String)  -- подтип («До погашения», …)
) ENGINE = MergeTree()
PARTITION BY Date(time)
ORDER BY (secid, time);
```

### Поля

| Поле | Тип | Описание |
|---|---|---|
| `time` | `DateTime` | дата торгового дня (`TRADEDATE`, 00:00 MSK) |
| `secid` | `String` | код инструмента, ключ для JOIN с `security_info` |
| `boardid` | `String` | борд: `TQCB` (корпоративные/биржевые), `TQOB` (ОФЗ/субфед/муниципальные), `TQDB` (еврооблигации) |
| `open`/`high`/`low`/`close` | `Float64` | цены, **чистые** (без НКД), в **% от номинала** |
| `value` | `Float64` | оборот в рублях |
| `volume` | `UInt64` | объём в лотах (для облигаций обычно = число бумаг) |
| `numtrades` | `UInt32` | число сделок за день |
| `accint` | `Float64` | НКД (накопленный купонный доход) на закрытие, в валюте номинала |
| `yieldclose` | `Float64` | доходность к погашению по цене закрытия, % |
| `yieldatwap` | `Float64` | доходность по средневзвешенной цене, % |
| `waprice` | `Float64` | средневзвешенная цена |
| `duration` | `Nullable(Float64)` | дюрация в днях; `NULL` у флоатеров/без даты погашения |
| `couponpercent` | `Float64` | ставка купона, % годовых |
| `couponvalue` | `Float64` | сумма купона на одну облигацию |
| `facevalue` | `Float64` | номинал (обычно 1000) |
| `faceunit` | `String` | валюта номинала (`SUR`/`RUB`, `USD`, …) |
| `currencyid` | `String` | валюта расчётов (`SUR`, `USD`, …) |
| `matdate` | `Nullable(Date)` | дата погашения; `NULL` при отсутствии |
| `bondtype` | `String` | тип: «Фикс с известным купоном», «Флоатер», «Валютные облигации», … |
| `bondsubtype` | `String` | подтип: «До погашения», «С офертой», … |

## Важные нюансы

### 1. Хранятся только торговавшиеся облигации

Эндпоинт history возвращает строку для **каждой** облигации борда, даже если по ней
не было ни одной сделки в этот день (у таких `close = 0`, `numtrades = 0`). Загрузчик
отбрасывает строки с `numtrades = 0` — иначе в таблицу попадало бы ~40–60% «мёртвых»
строк, засоряющих анализ цен. Если нужны «листингованные, но без сделок» — это
отдельный кейс, сейчас они не сохраняются.

### 2. `close`/`open` — чистая цена в % от номинала (без НКД)

`close` — это чистая цена (грязная цена минус НКД), выраженная в **процентах от номинала**,
а не в валюте. Например, `close = 78.087` при `facevalue = 1000` — это 780.87 ₽.
`accint` (НКД) хранится в **валюте номинала** (`faceunit`), поэтому складывать их напрямую
нельзя. Полная («грязная») цена в валюте = `close / 100 * facevalue + accint`.

### 3. Nullable-поля

- `duration` — `NULL` у флоатеров и бумаг без даты погашения.
- `matdate` — `NULL` при отсутствии даты погашения; значения-заглушки до 1970 года
  (например, `0001-01-01`) отбрасываются.

### 4. Один борд — один инструмент

Каждая облигация торгуется на одном основном борде (`TQCB`/`TQOB`/`TQDB`). Остальные
борды рынка облигаций (~36 шт.) — спецрежимы (аукционы, переговорные сделки и т.п.),
они не загружаются. В отдельные дни `TQDB` (еврооблигации) может быть пустым.

### 5. Инкрементальная загрузка

Как и в других `load-*` командах: последняя дата в таблице всегда перезагружается
(могла быть неполной), прочие даты с уже существующими строками пропускаются,
`--force` удаляет и перезагружает диапазон заново. Загрузка идёт по датам, market-wide
(~31 запрос на день вместо тысяч), поэтому полный бэкфилл за годы идёт быстро.

### 6. JOIN со справочником

Названия/эмитента в таблице нет — они в `tr.security_info`:

```sql
SELECT b.secid, i.shortname, i.emitent_title, b.close, b.yieldclose, b.accint
FROM tr.bond_daily b
JOIN tr.security_info i ON i.secid = b.secid
WHERE b.time >= now() - INTERVAL 7 DAY;
```

## Примеры запросов

Последние дневные свечи по ОФЗ:

```sql
SELECT time, secid, close, yieldclose, accint, duration, matdate
FROM tr.bond_daily
WHERE boardid = 'TQOB'
  AND time = (SELECT max(time) FROM tr.bond_daily)
ORDER BY yieldclose;
```

Динамика доходности конкретной бумаги:

```sql
SELECT time, close, yieldclose, accint, volume, numtrades
FROM tr.bond_daily
WHERE secid = 'SU26218RMFS6'
ORDER BY time DESC;
```

Облигации с наибольшей дюрацией (риск ставки):

```sql
SELECT secid, duration, yieldclose
FROM tr.bond_daily
WHERE time = (SELECT max(time) FROM tr.bond_daily)
ORDER BY duration DESC
LIMIT 20;
```

## Ссылки

- Эндпоинт history: https://iss.moex.com/iss/reference/ (см. `/iss/history/engines/stock/markets/bonds/securities`)
- Справочник инструментов: см. [security_info.md](./security_info.md)
