# Futures Trading Bot v4.0

Bot de trading automatizado para Binance Futures Testnet. Opera desde CLI, corre en la nube 24/7 con sesiones auto-rotativas de 1 hora. Toda la informacion (trades, sesiones, logs) se persiste en PostgreSQL (Supabase).

## Como funciona

El bot ejecuta una estrategia de **EMA Crossover con confirmacion RSI** sobre el par configurado (default XRPUSDT). Conecta al WebSocket de Binance Futures para recibir precios en tiempo real, calcula indicadores tecnicos y abre/cierra posiciones automaticamente aplicando gestion de riesgo.

### Estrategia

1. **Indicadores**: EMA rapida (9) y EMA lenta (21), RSI de 14 periodos, volatilidad
2. **Entrada LONG**: RSI cruza por encima de 40 + EMA rapida > EMA lenta + gate de volatilidad (precio movio >0.02% en 10 ticks)
3. **Entrada SHORT**: RSI cruza por debajo de 60 + EMA rapida < EMA lenta + gate de volatilidad
4. **Confirmacion**: La senal debe persistir `confirmation_count` ticks consecutivos (default 3)

### Gestion de riesgo

Cada posicion abierta tiene 4 mecanismos de salida, evaluados en cada tick:

| Mecanismo | Descripcion | Default |
|-----------|-------------|---------|
| **Stop Loss** | Cierra si el precio cae X% desde la entrada | 0.2% |
| **Take Profit** | Cierra si el precio sube X% desde la entrada | 0.5% |
| **Break Even** | Mueve el stop loss al precio de entrada cuando hay X% de ganancia | 0.1% |
| **Trailing Stop** | Despues del break-even, el stop sigue al precio a X% de distancia | 0.1% |
| **RSI Exhaustion** | Cierra longs si RSI > 70, shorts si RSI < 30 | - |

Ademas:
- **Cooldown**: Pausa de 60 ticks despues de un win o loss antes de operar de nuevo
- **Panic Protocol**: Si una perdida supera 1.5 USDT, el bot se apaga
- **Session Budget**: Cada trade usa exactamente 100 USDT de tamano de posicion
- **Una posicion a la vez**: No se abren multiples posiciones simultaneas

## Modos de operacion

### Run (24/7)

Lanza **todos los modos concurrentemente** con un solo comando. Diseñado para correr en la nube 24/7:

- **Scraper continuo**: Descarga klines cada hora automaticamente
- **Auto-backtest**: Se ejecuta cada vez que el scraper obtiene datos nuevos
- **Paper trading**: Sesiones auto-rotativas de 1hr
- **Testnet trading**: Sesiones auto-rotativas de 1hr. Si el budget se agota (liquidacion o margin call), se detiene y envia notificacion por Telegram

```bash
./trading-bot run
```

### Backtest

Ejecuta la estrategia sobre datos historicos sin riesgo. Puede usar datos de un archivo local (JSONL) o klines descargadas previamente con `scrape` y almacenadas en la base de datos.

Al terminar imprime un reporte con metricas: Win Rate, Profit Factor, Expectancy, Max Drawdown y Sharpe Ratio. Guarda la sesion y cada trade en la base de datos.

```bash
./trading-bot backtest                     # Usa klines de la DB
./trading-bot backtest data/archivo.jsonl  # Usa archivo local
```

### Paper Trading

Opera con dinero ficticio sobre precios reales de Binance en tiempo real. Es el modo recomendado para validar la estrategia antes de usar testnet.

Las sesiones se **auto-rotan**: cada `session_duration_min` minutos (default 60) la sesion termina, guarda analytics en la base de datos y arranca una sesion nueva automaticamente. Se detiene con Ctrl+C.

```bash
./trading-bot paper
```

### Testnet

Opera en la testnet real de Binance Futures. Envia ordenes de mercado reales contra `https://testnet.binancefuture.com` usando las API keys configuradas en `.env`. No usa dinero real, pero la ejecucion es identica a produccion.

Las sesiones se **auto-rotan**: cada `session_duration_min` minutos la sesion termina, guarda analytics y arranca una nueva. Si el budget se agota, se detiene y envia notificacion por Telegram.

```bash
./trading-bot testnet
```

### Scrape

Descarga klines historicas de la API publica de Binance Futures (`fapi.binance.com`) y las guarda en la base de datos. Funciona de forma **concurrente**: multiples simbolos en paralelo, y dentro de cada simbolo 3 requests simultaneos para descargar chunks de 500 velas.

Al terminar, si hay datos nuevos, ejecuta un backtest automaticamente sobre los datos descargados.

```bash
./trading-bot scrape                    # Descarga 24hs del simbolo en config.yaml
./trading-bot scrape BTCUSDT ETHUSDT    # Descarga multiples simbolos
```

## Todos los comandos

| Comando | Descripcion |
|---------|-------------|
| `run` | **Lanzar todos los modos concurrentes (24/7)**: scrape + paper + testnet + backtest |
| `backtest [archivo]` | Backtest sobre klines de la DB o archivo local JSONL |
| `paper` | Paper trading con sesiones auto-rotativas de 1hr |
| `testnet` | Testnet trading con sesiones auto-rotativas de 1hr |
| `scrape [simbolos...]` | Descargar klines historicas + auto-backtest |
| `trades` | Mostrar ultimos 20 trades de la base de datos |
| `sessions` | Mostrar ultimas 10 sesiones con metricas |
| `stats` | Mostrar estadisticas generales (win rate, PnL total) |
| `version` | Imprimir version |
| `help` | Mostrar ayuda |

## Configuracion

El bot se configura con dos archivos:

### config.yaml - Parametros de trading

```yaml
symbol: XRPUSDT              # Par de trading
leverage: 10                  # Apalancamiento
session_budget: 100.0         # USDT por trade (tamano de posicion)
ema_fast: 9                   # Periodo EMA rapida
ema_slow: 21                  # Periodo EMA lenta
min_ema_gap: 0.001            # Gap minimo entre EMAs
take_profit_pct: 0.5          # Take profit (%)
stop_loss_pct: 0.2            # Stop loss (%)
break_even_trigger_pct: 0.1   # Trigger para mover stop a break-even (%)
trail_distance_pct: 0.1       # Distancia del trailing stop (%)
confirmation_count: 3         # Ticks consecutivos para confirmar senal
min_profit_for_flip_exit: 0.1 # Profit minimo para salir por flip de indicador
paper_balance: 1000.0         # Balance inicial en paper trading
loss_cooldown: 60             # Ticks de espera despues de una perdida
win_cooldown: 60              # Ticks de espera despues de una ganancia
session_duration_min: 60      # Duracion de cada sesion en minutos
backtest_file: data/trades.jsonl  # Archivo default para backtest local
```

### .env - Secretos y conexiones

```bash
DATABASE_URL="host=db.xxx.supabase.co port=5432 user=postgres password=xxx dbname=postgres sslmode=require"
BINANCE_API_KEY=tu_api_key
BINANCE_SECRET_KEY=tu_secret_key
TELEGRAM_BOT_TOKEN=tu_bot_token       # Opcional
TELEGRAM_CHAT_ID=tu_chat_id           # Opcional
```

**Nota**: La DATABASE_URL usa formato DSN (key-value), no URL, porque los passwords con caracteres especiales rompen el URL parsing.

## Base de datos

Toda la informacion se persiste en PostgreSQL (Supabase). Las tablas se crean automaticamente al iniciar:

| Tabla | Contenido |
|-------|-----------|
| `trades` | Cada trade individual: symbol, side, entry, exit, profit, exit_reason |
| `sessions` | Cada sesion: modo, trades, W/L, PnL, profit factor, drawdown, sharpe, expectancy |
| `klines` | Velas historicas descargadas por el scraper: OHLCV + timestamps |
| `system_states` | Estado del sistema: paper balance actual |
| `trade_logs` | Logs de trades (reemplaza archivos JSONL) |
| `market_pulse_logs` | Snapshots de mercado |
| `bot_logs` | Logs generales del bot |

## Analytics

Cada sesion (backtest, paper, testnet) calcula y persiste:

- **Win Rate**: Porcentaje de trades ganadores
- **Profit Factor**: Ganancia bruta / perdida bruta (>1 = sistema rentable)
- **Expectancy**: PnL promedio por trade (cuanto esperas ganar/perder por operacion)
- **Max Drawdown**: Mayor caida acumulada desde un pico (peor racha de perdidas)
- **Sharpe Ratio**: Retorno ajustado por riesgo (media / desvio estandar de PnLs)

Ejemplo de salida de `trading-bot sessions`:

```
ID | Mode     | Symbol    | Trades | W/L    | PnL        | PF    | DD       | Sharpe | Status
1  | backtest | XRPUSDT   | 16     | 3/13   | -1.8358    | 0.36  | 2.1334   | -0.4774 | completed
```

## Arquitectura

```
trading-bot/
  main.go                CLI entry point (run, backtest, paper, testnet, scrape, etc.)
  config.yaml            Parametros de trading
  .env                   Secretos (DB, Binance, Telegram)
  Makefile               Build, test, deploy
  internal/
    analytics/           Calculo de metricas (PF, Sharpe, DD, Expectancy)
    backtest/            Motor de backtest sobre candles o archivos
    config/              Carga config.yaml + variables de entorno
    database/            PostgreSQL via GORM (trades, sessions, klines, logs)
    logging/             TradeCounter (in-memory session stats)
    marketdata/          Cliente Binance: REST (ordenes, balance) + WebSocket (precios)
    notifications/       Telegram notifier (o NullNotifier si no esta configurado)
    scraper/             Descarga concurrente de klines desde Binance API publica
    trading/
      domain/            Tipos core: Trade, Position, Signal, Candle, Status
      trading.go         EMACrossover, PositionManager, TradeTracker,
                         BinanceTrader (testnet), PaperTrader (simulado)
```

## Flujo de uso recomendado

```bash
# 1. Configurar .env y config.yaml

# 2. Descargar datos y validar estrategia
./trading-bot scrape              # Descarga 24hs + auto-backtest

# 3. Si el backtest es prometedor, lanzar todo 24/7
./trading-bot run                 # Scrape + paper + testnet + backtest concurrentes

# O correr modos por separado:
./trading-bot paper               # Solo paper con sesiones de 1hr
./trading-bot testnet             # Solo testnet con sesiones de 1hr

# 4. Monitorear resultados
./trading-bot stats               # Performance global
./trading-bot sessions            # Detalle por sesion
./trading-bot trades              # Trades individuales
```

## Logs

Todos los logs se persisten en Supabase PostgreSQL (tablas `trade_logs`, `market_pulse_logs`, `bot_logs`). No se usan archivos locales para logging.

## Deploy

```bash
make build     # Compilar binario optimizado
make test      # Correr tests
make vet       # Analisis estatico
make deploy    # Build + SCP al VM
make clean     # Limpiar artefactos
```

## Seguridad

- Precision decimal (`shopspring/decimal`) para todos los calculos financieros
- Mutex en todo estado compartido
- Hard shutdown en margin call o perdida > 1.5 USDT
- Enforcement de session budget
- Cooldown despues de wins/losses
- Una posicion a la vez
- Credenciales solo en `.env`, nunca en config.yaml
