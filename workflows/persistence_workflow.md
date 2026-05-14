# Goal

Implement a robust persistence layer for the trading bot.

# Requirements

1.  Define data models for core entities (e.g., Trades, Candles).
2.  Create a database interface for abstracting persistence logic.
3.  Provide a concrete implementation of the database interface.
4.  Ensure the persistence layer is used by the bot to store all relevant data.
5.  Add tests for the persistence layer.

# Constraints

- Follow the repository/service architecture.
- The persistence logic should be in its own package (`internal/database`).

# Deliverables

- `internal/database/database.go`: The database interface and implementation.
- `internal/database/models.go`: The data models.
- `internal/database/database_test.go`: Unit tests for the database.
- Integration with the main application logic.
