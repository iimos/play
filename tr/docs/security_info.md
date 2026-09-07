# security_info — справочник тикеров

Справочная таблица с метаданными ценных бумаг Московской биржи: названия,
эмитенты, коды, типы и группы инструментов. Нужна, чтобы в UI показывать
человекочитаемые названия вместо «голых» тикеров.

- Источник: MOEX ISS API — `https://iss.moex.com/iss/securities.json?q={тикер}`
- Наполнение: команда `go run main.go load-securities`
- Обновление: по требованию (см. раздел «Актуальность»)

## Таблица `tr.security_info`

```sql
CREATE TABLE tr.security_info (
    secid                LowCardinality(String),
    shortname            String, -- Краткое наименование
    name                 String, -- Полное наименование
    isin                 String,
    regnumber            String,
    is_traded            UInt8,
    emitent_id           String,
    emitent_title        String, -- Название эмитента
    emitent_inn          String,
    emitent_okpo         String,
    sec_type             LowCardinality(String), -- Тип бумаги
    sec_group            LowCardinality(String), -- Группа инструментов
    primary_boardid      LowCardinality(String),
    marketprice_boardid  LowCardinality(String),
    updated_at           DateTime DEFAULT now()
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY secid;
```

### Поля

| Поле | Тип | Описание |
|---|---|---|
| `secid` | `String` | код инструмента (тикер), ключ для JOIN |
| `shortname` | `String` | краткое название («Аэрофлот», «AI92-9.26», «USDRUB_TOM») |
| `name` | `String` | полное наименование («Аэрофлот-росс.авиалин(ПАО)ао») |
| `isin` | `String` | ISIN код (пустой у фьючерсов и валют) |
| `regnumber` | `String` | номер государственной регистрации |
| `is_traded` | `UInt8` | торгуется ли сейчас (1/0) |
| `emitent_id` | `String` | код эмитента (см. нюанс №4 про строковый тип) |
| `emitent_title` | `String` | полное название эмитента |
| `emitent_inn` | `String` | ИНН эмитента |
| `emitent_okpo` | `String` | ОКПО эмитента |
| `sec_type` | `String` | тип бумаги (`common_share`, `futures`, `currency`, ...) |
| `sec_group` | `String` | группа инструментов (`stock_shares`, `futures_forts`, ...) |
| `primary_boardid` | `String` | основной режим торгов (`TQBR`, `RFUD`, `CETS`, ...) |
| `marketprice_boardid` | `String` | режим торгов для рыночной цены |
| `updated_at` | `DateTime` | время последнего обновления записи |

## Важные нюансы

### 1. `ReplacingMergeTree` — в запросах нужен `FINAL`

Таблица хранит версии записей, дедупликация происходит асинхронно при слиянии
партов. До слияния по одному `secid` может лежать несколько строк (старая и новая).
Поэтому в запросах используйте `FINAL`:

```sql
SELECT * FROM tr.security_info FINAL WHERE secid = 'AFLT';
```

Без `FINAL` возможны дубликаты строк по `secid`.

### 2. Ключ — `secid`, но это не всегда тикер из API Algopack

`secid` совпадает с полем `secid` таблиц `super_eq`/`super_fo`/`super_fx` и с
`ticker` таблицы `candles`. JOIN идёт напрямую:

```sql
SELECT s.secid, i.shortname, i.emitent_title, s.pr_close
FROM tr.super_eq s
JOIN tr.security_info i ON i.secid = s.secid
WHERE s.time >= now() - INTERVAL 1 DAY;
```

Но это **не** `ticker` из `tr.futoi` и **не** `asset_code` из `super_fo` (см. `futoi.md`).

### 3. `super_eq` содержит не только акции

В `super_eq` попадают разные группы инструментов (по данным из `security_info`):

| `sec_group` | что это | пример |
|---|---|---|
| `stock_shares` | акции | `AFLT`, `SBER` |
| `stock_ppif` | биржевые ПИФы (фонды) | `exchange_ppif`, `private_ppif`, ... |
| `stock_dr` | депозитарные расписки | `depositary_receipt` |
| — (нет записи) | синтетические агрегаты | `BOND`, `CORP` |

Поэтому по `super_eq` нельзя предполагать, что все записи — обычные акции.

### 4. Поля эмитента пустые не у всех инструментов

`emitent_title`/`emitent_inn` заполнены только там, где есть эмитент:

- **Акции и ПИФы** — заполнены.
- **Поставочные/одиночные фьючерсы** на акции (`AFLT-9.26`, `ASTRM-3.27`) — заполнены
  (эмитент базового актива).
- **Фьючерсы на индексы/товары** (`AI92-9.26`, `RTS-9.26`, `AED-3.27`) — пустые.
- **Валюты и металлы** (`currency_selt`, `currency_metal`) — пустые.

В текущем наполнении ~585 из ~1376 записей имеют пустой `emitent_title`.

### 5. `emitent_id` хранится строкой

В ответе ISS `emitent_id` — это число (`711`) либо `null` (для фьючерсов/валют).
Поэтому в таблице поле `String`, чтобы вместить и пустое значение. Числовое
значение можно получить через `toInt64OrZero(emitent_id)`.

### 6. Синтетические агрегаты `BOND` и `CORP`

В `super_eq` есть записи с `secid = 'BOND'` и `'CORP'` — это агрегированные
строки (суммарно по облигациям / корпоративным бумагам), а не реальные
инструменты. В справочнике ISS их нет, поэтому `load-securities` их не найдёт и
в `security_info` они отсутствуют. При LEFT JOIN они получат `NULL`.

### 7. Данные могут устаревать

`load-securities` собирает тикеры из текущих данных и опрашивает ISS. Новые
фьючерсные контракты появляются на бирже регулярно (например, при листинге
очередной серии), поэтому после появления новых `secid` в `super_fo` их не будет
в `security_info`, пока не перезапущен `load-securities`. Список «отсутствующих»
можно получить так:

```sql
SELECT DISTINCT s.secid
FROM tr.super_fo s
LEFT ANTI JOIN tr.security_info i ON i.secid = s.secid;
```

### 8. Поиск в ISS — по подстроке, нужна фильтрация по точному `secid`

`?q=` ищет по коду/названию/ISIN и возвращает до 100 записей-совпадений
(`?q=AFLT` вернёт и `AFLT`, и `AFU6`, и `FIXAFLT`). Загрузчик берёт только
строку с точным совпадением `secid`, поэтому «не найден» ≠ «нет на бирже» —
это может означать, что инструмент уже снят с торгов (истёкший фьючерс).

### 9. `is_traded` — срез «сейчас»

Флаг отражает, торгуется ли инструмент на момент обновления. Истёкшие фьючерсы
и снятые с торгов бумаги будут иметь `is_traded = 0`, но их исторические данные
в `super_*` при этом остаются.

## Примеры запросов

Названия по списку тикеров:

```sql
SELECT secid, shortname, emitent_title
FROM tr.security_info FINAL
WHERE secid IN ('AFLT', 'SBER', 'USDRUB_TOM');
```

Свечи с человекочитаемыми названиями:

```sql
SELECT i.shortname, i.emitent_title, s.pr_close, s.vol
FROM tr.super_eq s
JOIN tr.security_info i ON i.secid = s.secid
WHERE s.time >= now() - INTERVAL 1 DAY
ORDER BY s.time DESC;
```

Все инструменты одной группы (акции):

```sql
SELECT secid, shortname, name
FROM tr.security_info FINAL
WHERE sec_group = 'stock_shares'
ORDER BY secid;
```

## Ссылки

- Описание эндпоинта: https://iss.moex.com/iss/reference/ (см. `/iss/securities`)
- Детальная спецификация инструмента: https://iss.moex.com/iss/securities/{secid}
- Справочник ISS: https://iss.moex.com/iss/reference/
