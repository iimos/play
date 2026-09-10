CREATE DATABASE tr;

CREATE TABLE tr.candles (
      time     DateTime CODEC(DoubleDelta(1), LZ4) COMMENT 'Начало свечки',
      ticker   LowCardinality(String) COMMENT 'Код инструмента (тикер)',
      open     Float32 COMMENT 'Цена открытия',
      close    Float32 COMMENT 'Цена закрытия',
      high     Float32 COMMENT 'Максимальная цена',
      low      Float32 COMMENT 'Минимальная цена',
      value    Float64 COMMENT 'Объем в рублях',
      volume   UInt64 COMMENT 'Объем в лотах'
) ENGINE = MergeTree()
COMMENT 'Свечи (OHLCV) по инструментам с торговых сессий'
PARTITION BY Date(time)
ORDER BY (ticker, time);

CREATE TABLE tr.super_eq (
      time          DateTime CODEC(DoubleDelta(1), LZ4) COMMENT 'Начало 5-минутного интервала',
      secid         LowCardinality(String) COMMENT 'Код инструмента (тикер)',

      -- tradestats: метрики сделок
      pr_open            Float32 COMMENT 'Цена открытия (первая сделка интервала)',
      pr_high            Float32 COMMENT 'Максимальная цена сделки за интервал',
      pr_low             Float32 COMMENT 'Минимальная цена сделки за интервал',
      pr_close           Float32 COMMENT 'Цена последней сделки за интервал',
      pr_std             Float32 COMMENT 'Стандартное отклонение цены (волатильность), %',
      vol                UInt32 COMMENT 'Объем в лотах',
      val                Float32 COMMENT 'Объем в рублях',
      trades             UInt32 COMMENT 'Количество сделок',
      pr_vwap            Float32 COMMENT 'Средневзвешенная цена (≈ val / vol)',
      pr_change          Float32 COMMENT 'Изменение цены за интервал, % = 100*(pr_close-pr_open)/pr_open',
      trades_b           UInt32 COMMENT 'Количество сделок на покупку',
      trades_s           UInt32 COMMENT 'Количество сделок на продажу',
      val_b              Float32 COMMENT 'Объем покупок в рублях',
      val_s              Float32 COMMENT 'Объем продаж в рублях',
      vol_b              UInt64 COMMENT 'Объем покупок в лотах',
      vol_s              UInt64 COMMENT 'Объем продаж в лотах',
      disb               Float32 COMMENT 'Дисбаланс покупатели/продавцы (1 — только покупки, 0 — баланс, -1 — только продажи)',
      pr_vwap_b          Float32 COMMENT 'Средневзвешенная цена покупок',
      pr_vwap_s          Float32 COMMENT 'Средневзвешенная цена продаж',
      sec_pr_open        UInt32 COMMENT 'Цена первой сделки в последнюю секунду интервала',
      sec_pr_high        UInt32 COMMENT 'Максимальная цена в последнюю секунду интервала',
      sec_pr_low         UInt32 COMMENT 'Минимальная цена в последнюю секунду интервала',
      sec_pr_close       UInt32 COMMENT 'Цена последней сделки в последнюю секунду интервала',

      -- obstats: метрики стакана заявок
      spread_bbo         Float32 COMMENT 'Спред лучших цен (best ask − best bid), базисные пункты',
      spread_lv10        Float32 COMMENT 'Спред на 10-м уровне стакана, базисные пункты',
      spread_1mio        Float32 COMMENT 'Спред на сделку в 1 млн ₽, базисные пункты',
      levels_b           UInt32 COMMENT 'Количество уровней цен на покупку',
      levels_s           UInt32 COMMENT 'Количество уровней цен на продажу',
      imbalance_vol_bbo  Float32 COMMENT 'Дисбаланс объема по лучшим ценам (bid vs ask)',
      imbalance_val_bbo  Float32 COMMENT 'Дисбаланс объема в рублях по лучшим ценам',
      imbalance_vol      Float32 COMMENT 'Дисбаланс объема по всему стакану',
      imbalance_val      Float32 COMMENT 'Дисбаланс объема в рублях по всему стакану',
      vwap_b             Float32 COMMENT 'Средневзвешенная цена заявок на покупку по всему стакану',
      vwap_s             Float32 COMMENT 'Средневзвешенная цена заявок на продажу по всему стакану',
      vwap_b_1mio        Float32 COMMENT 'Средневзвешенная цена покупки на сумму 1 млн ₽',
      vwap_s_1mio        Float32 COMMENT 'Средневзвешенная цена продажи на сумму 1 млн ₽',

      -- orderstats: метрики заявок
      put_orders_b       UInt32 COMMENT 'Количество заявок на покупку, выставленных в стакан',
      put_orders_s       UInt32 COMMENT 'Количество заявок на продажу, выставленных в стакан',
      put_val_b          Float32 COMMENT 'Стоимость выставленных заявок на покупку, ₽',
      put_val_s          Float32 COMMENT 'Стоимость выставленных заявок на продажу, ₽',
      put_vol_b          UInt32 COMMENT 'Объем выставленных заявок на покупку, лоты',
      put_vol_s          UInt32 COMMENT 'Объем выставленных заявок на продажу, лоты',
      put_vwap_b         Float32 COMMENT 'Средневзвешенная цена выставленных заявок на покупку',
      put_vwap_s         Float32 COMMENT 'Средневзвешенная цена выставленных заявок на продажу',
      put_vol            UInt32 COMMENT 'Суммарный объем выставленных заявок, лоты',
      put_val            Float32 COMMENT 'Суммарная стоимость выставленных заявок, ₽',
      put_orders         UInt32 COMMENT 'Суммарное количество выставленных заявок',
      cancel_orders_b    UInt32 COMMENT 'Количество снятых заявок на покупку',
      cancel_orders_s    UInt32 COMMENT 'Количество снятых заявок на продажу',
      cancel_val_b       Float32 COMMENT 'Стоимость снятых заявок на покупку, ₽',
      cancel_val_s       Float32 COMMENT 'Стоимость снятых заявок на продажу, ₽',
      cancel_vol_b       UInt32 COMMENT 'Объем снятых заявок на покупку, лоты',
      cancel_vol_s       UInt64 COMMENT 'Объем снятых заявок на продажу, лоты',
      cancel_vwap_b      Float32 COMMENT 'Средневзвешенная цена снятых заявок на покупку',
      cancel_vwap_s      Float32 COMMENT 'Средневзвешенная цена снятых заявок на продажу',
      cancel_vol         UInt64 COMMENT 'Суммарный объем снятых заявок, лоты',
      cancel_val         Float32 COMMENT 'Суммарная стоимость снятых заявок, ₽',
      cancel_orders      UInt64 COMMENT 'Суммарное количество снятых заявок'
) ENGINE = MergeTree()
COMMENT 'Суперсвечи (5-минутные) по акциям: tradestats + obstats + orderstats'
PARTITION BY Date(time)
ORDER BY (secid, time);

CREATE TABLE tr.super_fo (
       time          DateTime CODEC(DoubleDelta(1), LZ4) COMMENT 'Начало 5-минутного интервала',
       secid         LowCardinality(String) COMMENT 'Код инструмента (тикер контракта)',
       asset_code    LowCardinality(String) COMMENT 'Код базового актива (ASSETCODE)',

       -- tradestats: метрики сделок
       pr_open            Float32 COMMENT 'Цена открытия (первая сделка интервала)',
       pr_high            Float32 COMMENT 'Максимальная цена сделки за интервал',
       pr_low             Float32 COMMENT 'Минимальная цена сделки за интервал',
       pr_close           Float32 COMMENT 'Цена последней сделки за интервал',
       pr_std             Float32 COMMENT 'Стандартное отклонение цены (волатильность), %',
       vol                UInt32 COMMENT 'Объем в лотах',
       val                Float32 COMMENT 'Объем в рублях',
       trades             UInt32 COMMENT 'Количество сделок',
       pr_vwap            Float32 COMMENT 'Средневзвешенная цена (≈ val / vol)',
       pr_change          Float32 COMMENT 'Изменение цены за интервал, % = 100*(pr_close-pr_open)/pr_open',
       trades_b           UInt32 COMMENT 'Количество сделок на покупку',
       trades_s           UInt32 COMMENT 'Количество сделок на продажу',
       val_b              Float32 COMMENT 'Объем покупок в рублях',
       val_s              Float32 COMMENT 'Объем продаж в рублях',
       vol_b              UInt64 COMMENT 'Объем покупок в лотах',
       vol_s              UInt64 COMMENT 'Объем продаж в лотах',
       disb               Float32 COMMENT 'Дисбаланс покупатели/продавцы (1 — только покупки, 0 — баланс, -1 — только продажи)',
       pr_vwap_b          Float32 COMMENT 'Средневзвешенная цена покупок',
       pr_vwap_s          Float32 COMMENT 'Средневзвешенная цена продаж',
       im                 Float32 COMMENT 'Гарантийное обеспечение (initial margin), руб.',
       oi_open            UInt32 COMMENT 'Открытый интерес на начало интервала',
       oi_high            UInt32 COMMENT 'Максимальный открытый интерес за интервал',
       oi_low             UInt32 COMMENT 'Минимальный открытый интерес за интервал',
       oi_close           UInt32 COMMENT 'Открытый интерес на конец интервала',
       sec_pr_open        UInt32 COMMENT 'Цена первой сделки в последнюю секунду интервала',
       sec_pr_high        UInt32 COMMENT 'Максимальная цена в последнюю секунду интервала',
       sec_pr_low         UInt32 COMMENT 'Минимальная цена в последнюю секунду интервала',
       sec_pr_close       UInt32 COMMENT 'Цена последней сделки в последнюю секунду интервала',

       -- obstats: метрики стакана заявок
       mid_price      Float32 COMMENT 'Средняя цена между лучшей ценой покупки и продажи',
       micro_price    Float32 COMMENT 'Микропрайс — взвешенная цена с учётом дисбаланса на лучших уровнях',
       spread_l1      Float32 COMMENT 'Спред на 1-м уровне стакана, базисные пункты',
       spread_l2      Float32 COMMENT 'Спред на 2-м уровне стакана, базисные пункты',
       spread_l3      Float32 COMMENT 'Спред на 3-м уровне стакана, базисные пункты',
       spread_l5      Float32 COMMENT 'Спред на 5-м уровне стакана, базисные пункты',
       spread_l10     Float32 COMMENT 'Спред на 10-м уровне стакана, базисные пункты',
       spread_l20     Float32 COMMENT 'Спред на 20-м уровне стакана, базисные пункты',
       levels_b       UInt32 COMMENT 'Количество уровней цен на покупку',
       levels_s       UInt32 COMMENT 'Количество уровней цен на продажу',
       vol_b_l1       UInt64 COMMENT 'Совокупный объем заявок на покупку в 1 лучшем уровне',
       vol_b_l2       UInt64 COMMENT 'Совокупный объем заявок на покупку в 2 лучших уровнях',
       vol_b_l3       UInt64 COMMENT 'Совокупный объем заявок на покупку в 3 лучших уровнях',
       vol_b_l5       UInt64 COMMENT 'Совокупный объем заявок на покупку в 5 лучших уровнях',
       vol_b_l10      UInt64 COMMENT 'Совокупный объем заявок на покупку в 10 лучших уровнях',
       vol_b_l20      UInt64 COMMENT 'Совокупный объем заявок на покупку в 20 лучших уровнях',
       vol_s_l1       UInt64 COMMENT 'Совокупный объем заявок на продажу в 1 лучшем уровне',
       vol_s_l2       UInt64 COMMENT 'Совокупный объем заявок на продажу в 2 лучших уровнях',
       vol_s_l3       UInt64 COMMENT 'Совокупный объем заявок на продажу в 3 лучших уровнях',
       vol_s_l5       UInt64 COMMENT 'Совокупный объем заявок на продажу в 5 лучших уровнях',
       vol_s_l10      UInt64 COMMENT 'Совокупный объем заявок на продажу в 10 лучших уровнях',
       vol_s_l20      UInt64 COMMENT 'Совокупный объем заявок на продажу в 20 лучших уровнях',
       vwap_b_l3      Float32 COMMENT 'Средневзвешенная цена заявок на покупку в 3 лучших уровнях',
       vwap_b_l5      Float32 COMMENT 'Средневзвешенная цена заявок на покупку в 5 лучших уровнях',
       vwap_b_l10     Float32 COMMENT 'Средневзвешенная цена заявок на покупку в 10 лучших уровнях',
       vwap_b_l20     Float32 COMMENT 'Средневзвешенная цена заявок на покупку в 20 лучших уровнях',
       vwap_s_l3      Float32 COMMENT 'Средневзвешенная цена заявок на продажу в 3 лучших уровнях',
       vwap_s_l5      Float32 COMMENT 'Средневзвешенная цена заявок на продажу в 5 лучших уровнях',
       vwap_s_l10     Float32 COMMENT 'Средневзвешенная цена заявок на продажу в 10 лучших уровнях',
       vwap_s_l20     Float32 COMMENT 'Средневзвешенная цена заявок на продажу в 20 лучших уровнях'
) ENGINE = MergeTree()
COMMENT 'Суперсвечи (5-минутные) по фьючерсам: tradestats + obstats'
PARTITION BY Date(time)
ORDER BY (secid, time);

CREATE TABLE tr.super_fx (
     time          DateTime CODEC(DoubleDelta(1), LZ4) COMMENT 'Начало 5-минутного интервала',
     secid         LowCardinality(String) COMMENT 'Код инструмента (тикер)',

    -- tradestats: метрики сделок
     pr_open            Float32 COMMENT 'Цена открытия (первая сделка интервала)',
     pr_high            Float32 COMMENT 'Максимальная цена сделки за интервал',
     pr_low             Float32 COMMENT 'Минимальная цена сделки за интервал',
     pr_close           Float32 COMMENT 'Цена последней сделки за интервал',
     pr_std             Float32 COMMENT 'Стандартное отклонение цены (волатильность), %',
     vol                UInt64 COMMENT 'Объем в лотах',
     val                UInt64 COMMENT 'Объем в рублях',
     trades             UInt32 COMMENT 'Количество сделок',
     pr_vwap            Float32 COMMENT 'Средневзвешенная цена (≈ val / vol)',
     pr_change          Float32 COMMENT 'Изменение цены за интервал, % = 100*(pr_close-pr_open)/pr_open',
     trades_b           UInt32 COMMENT 'Количество сделок на покупку',
     trades_s           UInt32 COMMENT 'Количество сделок на продажу',
     val_b              Float32 COMMENT 'Объем покупок в рублях',
     val_s              Float32 COMMENT 'Объем продаж в рублях',
     vol_b              UInt64 COMMENT 'Объем покупок в лотах',
     vol_s              UInt64 COMMENT 'Объем продаж в лотах',
     disb               Float32 COMMENT 'Дисбаланс покупатели/продавцы (1 — только покупки, 0 — баланс, -1 — только продажи)',
     pr_vwap_b          Float32 COMMENT 'Средневзвешенная цена покупок',
     pr_vwap_s          Float32 COMMENT 'Средневзвешенная цена продаж',
     sec_pr_open        UInt32 COMMENT 'Цена первой сделки в последнюю секунду интервала',
     sec_pr_high        UInt32 COMMENT 'Максимальная цена в последнюю секунду интервала',
     sec_pr_low         UInt32 COMMENT 'Минимальная цена в последнюю секунду интервала',
     sec_pr_close       UInt32 COMMENT 'Цена последней сделки в последнюю секунду интервала',

    -- obstats: метрики стакана заявок
     mid_price      Float32 COMMENT 'Средняя цена между лучшей ценой покупки и продажи',
     micro_price    Float32 COMMENT 'Микропрайс — взвешенная цена с учётом дисбаланса на лучших уровнях',
     spread_l1      Float32 COMMENT 'Спред на 1-м уровне стакана, базисные пункты',
     spread_l2      Float32 COMMENT 'Спред на 2-м уровне стакана, базисные пункты',
     spread_l3      Float32 COMMENT 'Спред на 3-м уровне стакана, базисные пункты',
     spread_l5      Float32 COMMENT 'Спред на 5-м уровне стакана, базисные пункты',
     spread_l10     Float32 COMMENT 'Спред на 10-м уровне стакана, базисные пункты',
     levels_b       UInt32 COMMENT 'Количество уровней цен на покупку',
     levels_s       UInt32 COMMENT 'Количество уровней цен на продажу',
     vol_b_l1       UInt64 COMMENT 'Совокупный объем заявок на покупку в 1 лучшем уровне',
     vol_b_l2       UInt64 COMMENT 'Совокупный объем заявок на покупку в 2 лучших уровнях',
     vol_b_l3       UInt64 COMMENT 'Совокупный объем заявок на покупку в 3 лучших уровнях',
     vol_b_l5       UInt64 COMMENT 'Совокупный объем заявок на покупку в 5 лучших уровнях',
     vol_b_l10      UInt64 COMMENT 'Совокупный объем заявок на покупку в 10 лучших уровнях',
     vol_s_l1       UInt64 COMMENT 'Совокупный объем заявок на продажу в 1 лучшем уровне',
     vol_s_l2       UInt64 COMMENT 'Совокупный объем заявок на продажу в 2 лучших уровнях',
     vol_s_l3       UInt64 COMMENT 'Совокупный объем заявок на продажу в 3 лучших уровнях',
     vol_s_l5       UInt64 COMMENT 'Совокупный объем заявок на продажу в 5 лучших уровнях',
     vol_s_l10      UInt64 COMMENT 'Совокупный объем заявок на продажу в 10 лучших уровнях',
     vwap_b_l3      Float32 COMMENT 'Средневзвешенная цена заявок на покупку в 3 лучших уровнях',
     vwap_b_l5      Float32 COMMENT 'Средневзвешенная цена заявок на покупку в 5 лучших уровнях',
     vwap_b_l10     Float32 COMMENT 'Средневзвешенная цена заявок на покупку в 10 лучших уровнях',
     vwap_s_l3      Float32 COMMENT 'Средневзвешенная цена заявок на продажу в 3 лучших уровнях',
     vwap_s_l5      Float32 COMMENT 'Средневзвешенная цена заявок на продажу в 5 лучших уровнях',
     vwap_s_l10     Float32 COMMENT 'Средневзвешенная цена заявок на продажу в 10 лучших уровнях',

    -- orderstats: метрики заявок
     put_orders_b       UInt32 COMMENT 'Количество заявок на покупку, выставленных в стакан',
     put_orders_s       UInt32 COMMENT 'Количество заявок на продажу, выставленных в стакан',
     put_val_b          UInt64 COMMENT 'Стоимость выставленных заявок на покупку, ₽',
     put_val_s          UInt64 COMMENT 'Стоимость выставленных заявок на продажу, ₽',
     put_vol_b          UInt64 COMMENT 'Объем выставленных заявок на покупку, лоты',
     put_vol_s          UInt64 COMMENT 'Объем выставленных заявок на продажу, лоты',
     put_vwap_b         Float32 COMMENT 'Средневзвешенная цена выставленных заявок на покупку',
     put_vwap_s         Float32 COMMENT 'Средневзвешенная цена выставленных заявок на продажу',
     cancel_orders_b    UInt32 COMMENT 'Количество снятых заявок на покупку',
     cancel_orders_s    UInt32 COMMENT 'Количество снятых заявок на продажу',
     cancel_val_b       Float32 COMMENT 'Стоимость снятых заявок на покупку, ₽',
     cancel_val_s       Float32 COMMENT 'Стоимость снятых заявок на продажу, ₽',
     cancel_vol_b       UInt32 COMMENT 'Объем снятых заявок на покупку, лоты',
     cancel_vol_s       UInt64 COMMENT 'Объем снятых заявок на продажу, лоты',
     cancel_vwap_b      Float32 COMMENT 'Средневзвешенная цена снятых заявок на покупку',
     cancel_vwap_s      Float32 COMMENT 'Средневзвешенная цена снятых заявок на продажу'
) ENGINE = MergeTree()
COMMENT 'Суперсвечи (5-минутные) по валютам: tradestats + obstats + orderstats'
PARTITION BY Date(time)
ORDER BY (secid, time);

CREATE TABLE tr.futoi (
       time          DateTime CODEC(DoubleDelta(1), LZ4) COMMENT 'Момент среза (tradedate + tradetime)',
       ticker        LowCardinality(String) COMMENT 'Код базового актива (двухсимвольный или код вечного фьючерса)',
       clgroup       LowCardinality(String) COMMENT 'Группа клиентов: FIZ / YUR',
       pos           Int64 COMMENT 'Величина открытых позиций (нетто)',
       pos_long      Int64 COMMENT 'Величина длинных открытых позиций',
       pos_short     Int64 COMMENT 'Величина коротких открытых позиций (отрицательная)',
       pos_long_num  Int64 COMMENT 'Количество лиц с длинной позицией',
       pos_short_num Int64 COMMENT 'Количество лиц с короткой позицией'
) ENGINE = MergeTree()
COMMENT 'Открытые позиции по фьючерсам в разрезе физ/юр лиц (FUTOI, 5-минутные срезы)'
PARTITION BY Date(time)
ORDER BY (ticker, clgroup, time);

CREATE TABLE tr.iss_openpositions (
       time                 DateTime CODEC(DoubleDelta(1), LZ4) COMMENT 'Дата торгового дня (tradedate, 00:00)',
       asset                LowCardinality(String) COMMENT 'ASSETCODE базового актива (полная нотация, напр. AFKS, ASTR)',
       clgroup              LowCardinality(String) COMMENT 'Группа клиентов: FIZ / YUR (из is_fiz: 0=YUR, 1=FIZ)',
       persons_long         Int64 COMMENT 'Количество лиц с длинной позицией',
       persons_short        Int64 COMMENT 'Количество лиц с короткой позицией',
       open_position_long   Int64 COMMENT 'Величина длинных открытых позиций',
       open_position_short  Int64 COMMENT 'Величина коротких открытых позиций (положительная)',
       oichange_long        Int64 COMMENT 'Изменение ОИ по лонгам',
       oichange_short       Int64 COMMENT 'Изменение ОИ по шортам'
) ENGINE = MergeTree()
COMMENT 'Дневные открытые позиции по фьючерсам в разрезе физ/юр лиц (ISS statistics)'
PARTITION BY Date(time)
ORDER BY (asset, clgroup, time);

CREATE TABLE tr.bond_daily (
       time          DateTime CODEC(DoubleDelta(1), LZ4) COMMENT 'Дата торгового дня (TRADEDATE, 00:00)',
       secid         LowCardinality(String) COMMENT 'Код инструмента (тикер)',
       boardid       LowCardinality(String) COMMENT 'Режим торгов (TQCB/TQOB/TQDB)',
       open          Float64 COMMENT 'Цена открытия',
       high          Float64 COMMENT 'Максимальная цена',
       low           Float64 COMMENT 'Минимальная цена',
       close         Float64 COMMENT 'Цена закрытия',
       value         Float64 COMMENT 'Объем в рублях',
       volume        UInt64 COMMENT 'Объем в лотах/штуках',
       numtrades     UInt32 COMMENT 'Количество сделок',
       accint        Float64 COMMENT 'Накопленный купонный доход (НКД)',
       yieldclose    Float64 COMMENT 'Доходность к погашению по цене закрытия, %',
       yieldatwap    Float64 COMMENT 'Доходность по средневзвешенной цене, %',
       waprice       Float64 COMMENT 'Средневзвешенная цена',
       duration      Nullable(Float64) COMMENT 'Дюрация, дней',
       couponpercent Float64 COMMENT 'Ставка купона, %',
       couponvalue   Float64 COMMENT 'Сумма купона на облигацию',
       facevalue     Float64 COMMENT 'Номинал',
       faceunit      LowCardinality(String) COMMENT 'Валюта номинала',
       currencyid    LowCardinality(String) COMMENT 'Валюта расчетов',
       matdate       Nullable(Date) COMMENT 'Дата погашения',
       bondtype      LowCardinality(String) COMMENT 'Тип облигации',
       bondsubtype   LowCardinality(String) COMMENT 'Подтип облигации'
) ENGINE = MergeTree()
PARTITION BY Date(time)
ORDER BY (secid, time)
COMMENT 'Дневные свечи облигаций с облигационными атрибутами (ISS history)';

CREATE TABLE tr.security_info (
    secid                LowCardinality(String) COMMENT 'Код инструмента (тикер), ключ для JOIN',
    shortname            String COMMENT 'Краткое наименование',
    name                 String COMMENT 'Полное наименование',
    isin                 String COMMENT 'ISIN код (пустой у фьючерсов и валют)',
    regnumber            String COMMENT 'Номер государственной регистрации',
    is_traded            UInt8 COMMENT 'Торгуется ли инструмент на момент обновления (1/0)',
    emitent_id           String COMMENT 'Код эмитента (строкой, может быть пустым)',
    emitent_title        String COMMENT 'Название эмитента',
    emitent_inn          String COMMENT 'ИНН эмитента',
    emitent_okpo         String COMMENT 'ОКПО эмитента',
    sec_type             LowCardinality(String) COMMENT 'Тип бумаги (common_share, futures, currency, ...)',
    sec_group            LowCardinality(String) COMMENT 'Группа (stock_shares, futures_forts, currency_selt, ...)',
    primary_boardid      LowCardinality(String) COMMENT 'Основной режим торгов (TQBR, RFUD, CETS, ...)',
    marketprice_boardid  LowCardinality(String) COMMENT 'Режим торгов для рыночной цены',
    lotsize              Float64 COMMENT 'Размер лота (LOTSIZE; для фьючерсов LOTVOLUME)',
    trading_currency     LowCardinality(String) COMMENT 'Валюта торгов (CURRENCYID; у фьючерсов пусто)',
    decimals             UInt8 COMMENT 'Число знаков после запятой в цене',
    minstep              Float64 COMMENT 'Минимальный шаг цены',
    facevalue            Float64 COMMENT 'Номинальная стоимость (FACEVALUE)',
    faceunit             LowCardinality(String) COMMENT 'Валюта номинала (FACEUNIT)',
    asset_code           LowCardinality(String) COMMENT 'Базовый актив фьючерса (ASSETCODE)',
    last_tradedate       Nullable(Date) COMMENT 'Дата последней торговли фьючерса (LASTTRADEDATE)',
    last_deldate         Nullable(Date) COMMENT 'Дата исполнения фьючерса (LASTDELDATE)',
    updated_at           DateTime DEFAULT now() COMMENT 'Время последнего обновления записи'
) ENGINE = ReplacingMergeTree(updated_at)
COMMENT 'Справочник метаданных ценных бумаг Московской биржи'
ORDER BY secid;
