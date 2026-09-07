# FUTOI — открытые позиции по фьючерсам

Датасет **FUTOI (Futures Open Interest)** — открытые позиции по фьючерсным контрактам
Московской биржи с детализацией по типам участников рынка.

- Источник: MOEX AlgoPack — `https://moexalgo.github.io/docs/description/futoi/`
- Периодичность обновления: каждые **5 минут**
- Охват данных: **все серии контрактов, агрегированные по базовому активу**
  (например, для `Si` суммируются `SiZ5`, `SiH6`, `SiM6` и т.д.)
- Сегментация: физические (`FIZ`) и юридические (`YUR`) лица
- Глубина истории: с **2020-01-03**

## Таблица `tr.futoi`

```sql
CREATE TABLE tr.futoi (
    time          DateTime CODEC(DoubleDelta(1), LZ4), -- tradedate + tradetime
    ticker        LowCardinality(String), -- код базового актива (старая нотация)
    clgroup       LowCardinality(String), -- группа клиентов: FIZ / YUR
    pos           Int64, -- нетто-позиция (long + short)
    pos_long      Int64, -- величина длинных открытых позиций
    pos_short     Int64, -- величина коротких открытых позиций (ОТРИЦАТЕЛЬНАЯ)
    pos_long_num  Int64, -- количество лиц с длинной позицией
    pos_short_num Int64  -- количество лиц с короткой позицией
) ENGINE = MergeTree()
PARTITION BY Date(time)
ORDER BY (ticker, clgroup, time);
```

### Поля

| Поле | Тип | Описание |
|---|---|---|
| `time` | `DateTime` | момент среза (дата + время торговли, MSK) |
| `ticker` | `String` | код базового актива фьючерса |
| `clgroup` | `String` | группа клиентов: `FIZ` / `YUR` |
| `pos` | `Int64` | нетто-позиция, `pos_long + pos_short` |
| `pos_long` | `Int64` | суммарные длинные позиции (контракты) |
| `pos_short` | `Int64` | суммарные короткие позиции (контракты), **отрицательное значение** |
| `pos_long_num` | `Int64` | число участников с длинной позицией |
| `pos_short_num` | `Int64` | число участников с короткой позицией |

## Важные нюансы

### 1. `ticker` — это код базового актива, а не `secid`

`ticker` агрегирует все контракты одного базового актива. Это не `secid` отдельного
контракта (не `SiZ5`, а `Si`) и **не совпадает** с `asset_code` из `super_fo`.

FUTOI использует старую нотацию кодов базовых активов:

| FUTOI `ticker` | `super_fo.asset_code` / ISS `ASSETCODE` |
|---|---|
| `RI` | `RTS` |
| `GZ` | `GAZR` |
| `LK` | `LKOH` |
| `MG` | `MGNT` |
| `USDRUBF` | `USDRUBTOM` |
| `EURRUBF` | `EURRUBTOM` |
| `CNYRUBF` | `CNYRUBTOM` |
| `GLDRUBF` | `GLDRUBTOM` |

При джойне с `super_fo` это нужно учитывать (прямого ключа нет).

### 2. `pos_short` отрицательный

`pos_short` хранит короткие позиции со знаком минус. Если нужен «объём в шорте» —
брать `abs(pos_short)`. `pos` — уже нетто-сумма.

### 3. Неравномерная сетка срезов

Данные идут не строго каждые 5 минут — есть технологические перерывы и пропуски
(например, 07:55 → 09:00, перерывы в клиринге). Пропуски в сетке — это норма, а не
дыры в данных.

### 4. Вечные фьючерсы заканчивают сессию раньше

Вечные фьючерсы (`USDRUBF`, `EURRUBF`, `CNYRUBF`, `GLDRUBF`, `IMOEXF`, `SBERF`,
`GAZPF` и др.) заканчивают вечернюю сессию в **23:45**, тогда как обычные — в **23:50**.
Это не баг.

### 5. Нет отдельного поля `secid`/`asset_code`

В FUTOI нет привязки к конкретному контракту — только агрегация по базовому активу
и группе клиентов. Для привязки к метаданным можно использовать справочник
`tr.security_info`, но связь с `ticker` не прямая.

## Неполное покрытие данных

FUTOI покрывает **не все** фьючерсные активы. У части акций есть фьючерсы, но данных
FUTOI по ним нет вовсе (например, `IS`/Артген, `AK`/Система, `AS`/Астра, `BN`/Башнефть,
`NB`/НоваБев, `BS`/БСП, `CM`/МКБ — всего ~45 бумаг). В таких случаях используйте
альтернативный источник — ISS statistics `openpositions`.

### ISS statistics `openpositions`

- Таблица: `tr.iss_openpositions`
- Команда загрузки: `go run main.go load-iss-openpositions`
- Эндпоинт: `https://iss.moex.com/iss/statistics/engines/futures/markets/forts/openpositions/{ASSETCODE}.json`
- Детали: см. [iss_openpositions.md](./iss_openpositions.md)

Ключевые отличия от FUTOI:

| | `tr.futoi` | `tr.iss_openpositions` |
|---|---|---|
| granularity | 5 минут | 1 день (`tradedate`) |
| ключ актива | короткий код (`IS`, `AK`, `BN`…) | полный `ASSETCODE` (`ISKJ`, `AFKS`, `BANE`…) |
| группа клиентов | `clgroup` (FIZ/YUR) | `clgroup` (FIZ/YUR, из `is_fiz`) |
| покрытие | неполное | все активы FORTS |

Прямого ключа между таблицами нет: `ticker` (FUTOI) и `asset` (openpositions) — разные
нотации кода базового актива, и маппинг нужно вести вручную (см. таблицу соответствия
в `./iss_openpositions.md`).

## Примеры запросов

Последний снимок по всем инструментам:

```sql
SELECT ticker, clgroup, pos, pos_long, pos_short, pos_long_num, pos_short_num
FROM tr.futoi
WHERE Date(time) = (SELECT max(Date(time)) FROM tr.futoi)
ORDER BY ticker, clgroup;
```

Динамика позиций физлиц по доллару-рублю за день:

```sql
SELECT time, pos_long, pos_short, pos_long_num, pos_short_num
FROM tr.futoi
WHERE ticker = 'Si' AND clgroup = 'FIZ' AND Date(time) = '2026-09-04'
ORDER BY time;
```

Дисбаланс физлиц по инструментам (последний снимок):

```sql
SELECT ticker, pos_long, abs(pos_short) AS short_abs,
       pos_long - abs(pos_short) AS imbalance
FROM tr.futoi
WHERE clgroup = 'FIZ'
  AND Date(time) = (SELECT max(Date(time)) FROM tr.futoi)
ORDER BY imbalance DESC;
```

Средний размер позиции на одного участника (концентрация):

```sql
SELECT ticker, clgroup,
       round(pos_long / pos_long_num, 1)   AS avg_long_per_person,
       round(abs(pos_short) / pos_short_num, 1) AS avg_short_per_person
FROM tr.futoi
WHERE Date(time) = (SELECT max(Date(time)) FROM tr.futoi)
  AND pos_long_num > 0
ORDER BY ticker, clgroup;
```

## Ссылки

- Описание датасета: https://moexalgo.github.io/docs/description/futoi/
- Методология расчёта: https://moexalgo.github.io/docs/method/futoi/
