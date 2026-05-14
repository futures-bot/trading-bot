CREATE TABLE trades (
    id SERIAL PRIMARY KEY,
    time TIMESTAMPTZ NOT NULL,
    side TEXT NOT NULL,
    entry_price DECIMAL NOT NULL,
    exit_price DECIMAL NOT NULL,
    pnl_usdt DECIMAL NOT NULL,
    balance DECIMAL NOT NULL
);

CREATE TABLE positions (
    id SERIAL PRIMARY KEY,
    symbol TEXT NOT NULL,
    side TEXT NOT NULL,
    size DECIMAL NOT NULL,
    entry_price DECIMAL NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE fills (
    id SERIAL PRIMARY KEY,
    trade_id INTEGER REFERENCES trades(id),
    price DECIMAL NOT NULL,
    quantity DECIMAL NOT NULL,
    fee DECIMAL NOT NULL,
    fee_currency TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE signals (
    id SERIAL PRIMARY KEY,
    time TIMESTAMPTZ NOT NULL,
    signal TEXT NOT NULL,
    fast_ema DECIMAL,
    slow_ema DECIMAL,
    atr DECIMAL,
    rsi DECIMAL,
    volume_ratio DECIMAL,
    market_regime TEXT,
    reason TEXT
);

CREATE TABLE candles (
    id SERIAL PRIMARY KEY,
    symbol TEXT NOT NULL,
    time TIMESTAMPTZ NOT NULL,
    open DECIMAL NOT NULL,
    high DECIMAL NOT NULL,
    low DECIMAL NOT NULL,
    close DECIMAL NOT NULL,
    volume DECIMAL NOT NULL
);

CREATE TABLE indicators (
    id SERIAL PRIMARY KEY,
    candle_id INTEGER REFERENCES candles(id),
    ema9 DECIMAL,
    ema21 DECIMAL,
    atr14 DECIMAL,
    rsi14 DECIMAL
);

CREATE TABLE balance_snapshots (
    id SERIAL PRIMARY KEY,
    time TIMESTAMPTZ NOT NULL,
    balance DECIMAL NOT NULL,
    currency TEXT NOT NULL
);

CREATE TABLE funding_fees (
    id SERIAL PRIMARY KEY,
    time TIMESTAMPTZ NOT NULL,
    symbol TEXT NOT NULL,
    fee DECIMAL NOT NULL
);

CREATE TABLE liquidation_prices (
    id SERIAL PRIMARY KEY,
    position_id INTEGER REFERENCES positions(id),
    price DECIMAL NOT NULL
);

CREATE TABLE strategy_decisions (
    id SERIAL PRIMARY KEY,
    time TIMESTAMTz NOT NULL,
    decision TEXT NOT NULL,
    reason TEXT
);

CREATE TABLE rejected_orders (
    id SERIAL PRIMARY KEY,
    time TIMESTAMPTZ NOT NULL,
    reason TEXT NOT NULL
);