# План: пересборка `super_index` при `load` и в `load --watch`

## Цель

Пересчитывать индексные суперсвечи (`tr.super_index`, синтез из `super_eq` +
`index_weights` + `index_candles` + `super_fx`) сразу после загрузки их
источников, а также обновлять их в лайве в `load --watch` (каждые 5 минут,
после обновления источников).

## Контекст / что мешает сейчас

1. `build-index-super` (`cmd/indexsuper`) нигде не вызывается из `load`:
   `load` в `main.go:161` параллельно запускает `supercandles.LoadAll`, `futoi`,
   `openpositions`, `bonddaily`, `indexdata.Load`, а синтез не зовёт.
2. Порядок зависимостей: `buildIndexDay` читает `super_eq`, `super_fx`,
   `index_candles` (`IndexDailyClose`, `IndexIntradaySession`), `index_weights`
   (`PrevTradingDate`, `IndexWeightsForDate`). Нужны веса **предыдущего**
   торгового дня — они стабильны внутри дня.
3. Все загрузчики при `Watch:true` входят в бесконечный цикл внутри своего
   `Load` (`superstocks.go:121`, `indexdata.go:136`, …), поэтому `gr.Wait()` в
   `load` не возвращается — «после них» без оркестрации собрать нельзя.
4. Окно сессии текущего дня берётся из 10-минутных свечей индекса
   (`IndexIntradaySession`), которые `load-index` обновляет раз в час. При
   5-минутной пересборке свежие бары `super_eq` обрезаются до времени последнего
   часового релоада.
5. `DeleteSuperIndexFor` + insert неатомарны и на 42 индексах дают ~42
   синхронные мутации за такт.

## Выбранный подход: Вариант B (callbacks)

Существующие watch-циклы источников сохраняют свои `watch.Tracker`
(rollover/backfill работают как раньше). После каждого успешного `ReloadDay`
они дёргают хук, который передаёт день в **debounced, mutex-guarded** `Builder`
из `cmd/indexsuper`. Builder копит дни и собирает `super_index` одной
сериализованной операцией.

Отвергнутые варианты:
- Разделение фаз + отдельный оркестратор в `cmd/load`: дороже, требует
  дублировать семантику `Tracker` и вводить общий сериализованный scheduler.
- Независимый 5-мин цикл со смещением: не гарантирует, что веса/свечи индекса
  обновлены до сборки.

## Изменения по файлам

### 1. `cmd/indexsuper` — Builder + фиксы окна/гейта

- `Builder`:
  - `NewBuilder(ctx, indices) (*Builder, error)` — один раз резолвит
    `moexindex.Resolve` + `moexindex.Currencies` + `http.Client`; создаёт
    `store.Store`.
  - `Trigger(d time.Time)` — неблокирующе кладёт день в set под mutex.
  - `Run(ctx) error` — worker: копит триггеры, ждёт короткий debounce
    (~15–30с тишины), забирает снапшот дней, зовёт `BuildDays(ctx, days, true)`.
    Сериализует сборки (исключает наложение с drop→insert источников и между
    собой).
  - `BuildDays(ctx, days, force)` — вынести логику цикла `buildDay`; **без**
    `runtime.GC()` на каждый такт (нит N2).
- `Build(ctx, opts)` для команды `build-index-super` остаётся как есть (история +
  `RunHourly`); `lastByIndex` считать на вызов, не кэшировать (D1).
- **Окно сессии (B2)** в `buildIndexDay` для `d == today`:
  - `sessStart` — из `IndexIntradaySession`, иначе фолбэк 09:00;
  - `end = lastPresentBarTime + 5m` (по факту наличия баров конституентов,
    ≤ `now.Truncate(5m)`), а **не** `max(indexEnd, now)`. Нет будущих и
    после-close фантомных плоских строк, лайв не режется часовым лагом.
  - если `now <= sessStart` — ноль строк.
- **Гейт текущего дня (B3):** строить только при доказательстве торгов —
  `IndexIntradaySession ok` **и** хотя бы один бар конституента в окне.
  `IndexDailyClose(today)` больше не обязателен (используется лишь в `validate`,
  `indexsuper.go:619-625` — для сегодня пропустить). Для прошедших дней
  поведение прежнее.
- Пропуски логировать явно: нет весов/якоря за `prevDate`, нет интрадея, нет
  баров.

### 2. `cmd/supercandles` — хук

- В `LoadOptions` добавить `AfterReload func(day time.Time)`.
- `LoadStocks` → `watchEq(ctx, storage, sess, after)`;
  `LoadCurrencies` → `watchFX(..., after)`. В цикле после успешного
  `watch.ReloadDay` вызвать `after(d)` (неблокирующий `Trigger`).
- `LoadFutures`/`watchFO` хук не нужен (на `super_index` не влияет).
- Опционально (рекомендуется): в конце первичной загрузки, перед входом в
  watch, вызвать `after(end)` один раз, чтобы первый build случился сразу, а не
  через ≤5 мин.

### 3. `cmd/indexdata` — хук

- `LoadOptions.AfterReload func(day time.Time)`; `watchIndexData(ctx, storage,
  client, infos, after)` — вызвать `after(d)` после `loadDay`.

### 4. `main.go` — `load`

- **Без watch:** как сейчас параллельно `Watch:false` → `gr.Wait()` →
  `indexsuper.Build` (только если `err == nil`).
- **С watch:** до старта создать `builder, err := indexsuper.NewBuilder(ctx,
  indices)`; при ошибке резолва — залогировать и продолжить без сборки.
  Запустить `builder.Run` в errgroup; проставить `AfterReload =
  builder.Trigger` в `supercandles.LoadOptions` и `indexdata.LoadOptions`;
  запустить лоадеры с `Watch:true`. `gr.Wait()` (до отмены ctx).
  `futoi`/`openpositions`/`bonddaily` — свои циклы, хук не нужен.
- Ошибка лоадера в phase-1 при watch: текущая семантика errgroup (отмена всех)
  сохраняется.
- `--index` для `load` по-прежнему игнорируется (builder собирает все).

### 5. Атомарность/стоимость (C2/D2)

- По умолчанию **оставляем per-index** `DeleteSuperIndexFor` + `StoreSuperIndex`
  (безопасно: не теряем дни, где индекс временно пропущен). Цена ~42
  `mutations_sync=2`/такт.
- Опционально: добавить `Store.DropSuperIndexPartition(day)` и при полной сборке
  дня использовать `DROP PARTITION` + один insert — быстрее и почти атомарно,
  но при пропуске какого-то индекса строки дня для него исчезнут. Вынести
  отдельным решением; по умолчанию не трогаем.

### 6. Доки/тесты

- `docs/index_super.md` (лейв раз в 5 мин через `load --watch`, порядок
  источники→синтез, окно текущего дня), `AGENTS.md`, usage в `main.go`.
- Тесты: окно сессии (после закрытия / до открытия / выходные / нет интрадея),
  debounce/coalesce Builder, смена дня через хук.

## Проверка

- `go build ./...`, `go vet ./...`.
- `go run main.go load --watch`: после тика eq/fx в логе есть build
  `super_index`; `max(time)` в `super_index` ≈ `max(time)` в `super_eq` за
  сегодня; валютные индексы (`RTS*`) появляются после заливки `super_fx`
  (сейчас `super_fx` за сегодня пуст — побочный симптом).
- Вручную: до открытия — сегодняшних строк нет; после закрытия `super_index` не
  растёт; в выходной — нет плоских 120 строк.

## Замечания ревью (учтены)

- **B1 (блокер):** нельзя терять `watch.Tracker` — Вариант B его сохраняет.
- **B2 (блокер):** `max(indexEnd, now)` рождает фантомные строки → заменено на
  клампинг по последнему бару.
- **B3 (блокер):** ослабление гейта `IndexDailyClose` строит неторговые дни →
  гейт по `IndexIntradaySession ok` + наличию баров.
- **C1:** сериализация через mutex внутри `Builder` (один worker), а не два
  независимых таймера.
- **C2/D2:** неатомарность и стоимость delete+insert — см. п.5.
- **C3:** семантика ошибок phase-1 и `Indices` — build пропускается при ошибке,
  `--index` для `load` не используется.
- **D1:** `lastByIndex` не кэшировать.
- **N2:** `runtime.GC()` не гонять каждые 5 минут.
- **N3:** не вводить экспортируемый `ReloadDay` в `indexdata` без необходимости.
