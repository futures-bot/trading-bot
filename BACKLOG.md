# Backlog

This file contains a list of future tasks and improvements for the `futures-test` application.

## High Priority

- [ ] **Implement Risk Engine**: Separate risk management from execution logic. The risk engine should validate max leverage, max exposure, drawdown protection, and cooldowns.
- [ ] **Build Replay System**: Create a system for replaying trading sessions from persisted data. This will be crucial for debugging and analysis.

## Medium Priority

- [ ] **Refactor to Incremental Analytics**: Migrate analytics calculations to incremental processing to improve performance and scalability.
- [ ] **Add More Strategies**: Implement and test new trading strategies.
- [ ] **Improve Backtester**: Enhance the backtester to provide more detailed statistics and visualizations.

## Low Priority

- [ ] **Add Support for More Exchanges**: Abstract the exchange-specific logic to allow for easy integration with other exchanges.
- [ ] **Implement Live Trading**: Add support for live trading with real funds.
