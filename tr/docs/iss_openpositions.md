# openpositions — дневные открытые позиции по фьючерсам (ISS statistics)

Источник: ISS statistics `openpositions` — ежедневные открытые позиции по фьючерсам
Московской биржи в разрезе физических и юридических лиц.

- Эндпоинт: `https://iss.moex.com/iss/statistics/engines/futures/markets/forts/openpositions/{ASSETCODE}.json`
- Периодичность: **ежедневный срез** (одна строка на актив/группу за торговый день)
- Сегментация: физические (`FIZ`) и юридические (`YUR`) лица
- Охват: все фьючерсные активы FORTS (включая те, которых нет в FUTOI)

## Отличие от FUTOI

FUTOI (AlgoPack, таблица tr.futoi) — 5-минутные срезы по короткому коду базового
актива. openpositions — дневной срез по полному ASSETCODE.

| | tr.futoi | tr.iss_openpositions |
|---|---|---|
| granularity | 5 минут | 1 день |
| ключ актива | короткий код (`IS`, `AK`, `BN`…) | полный `ASSETCODE` (`ISKJ`, `AFKS`, `BANE`…) |
| поле группы | `clgroup` (FIZ/YUR) | `clgroup` (FIZ/YUR, из `is_fiz`) |

See ./futoi.md

## Таблица `tr.iss_openpositions`

```sql
CREATE TABLE tr.iss_openpositions (
    time                 DateTime, -- tradedate (день, 00:00)
    asset                LowCardinality(String), -- ASSETCODE базового актива
    clgroup              LowCardinality(String), -- FIZ / YUR
    persons_long         Int64, -- количество лиц с длинной позицией
    persons_short        Int64, -- количество лиц с короткой позицией
    open_position_long   Int64, -- длинные открытые позиции
    open_position_short  Int64, -- короткие открытые позиции (положительное)
    oichange_long        Int64, -- изменение ОИ по лонгам
    oichange_short       Int64  -- изменение ОИ по шортам
) ENGINE = MergeTree()
PARTITION BY Date(time)
ORDER BY (asset, clgroup, time);
```

### Соответствие полям FUTOI

| openpositions | FUTOI |
|---|---|
| `persons_long` | `pos_long_num` |
| `persons_short` | `pos_short_num` |
| `open_position_long` | `pos_long` |
| `open_position_short` | `abs(pos_short)` |
| `oichange_long` / `oichange_short` | — (в FUTOI нет) |

Нетто-позиция: `open_position_long - open_position_short`.

## Маппинг `asset` → FUTOI ticker

`asset` использует полную нотацию `ASSETCODE`, которая в большинстве случаев совпадает
с `secid` акции, но бывают исключения:

| `asset` (ISS) | FUTOI ticker | бумага |
|---|---|---|
| `ISKJ` | `IS` | Артген (ABIO) |
| `AFKS` | `AK` | Система (AFKS) |
| `ASTR` | `AS` | Астра (ASTR) |
| `BANE` | `BN` | Башнефть (BANE/BANEP) |
| `BELUGA` | `NB` | НоваБев (BELU) |
| `BSPB` | `BS` | БСП (BSPB/BSPBP) |
| `CBOM` | `CM` | МКБ (CBOM) |
| `RTS` | `RI` | индекс РТС |
| `GAZR` | `GZ` | Газпром |
