# Goal

Standardize all NATS events across the system.

# Requirements

1.  Define a common event structure with `event_type`, `version`, `timestamp`, `source`, and `payload`.
2.  Use a shared package for event definitions.
3.  Ensure all published events conform to the standard schema.
4.  Provide serialization and deserialization helpers.

# Constraints

- All events must be defined in the `shared/events` package.
- All event publishing must go through a centralized `Publisher` that enforces the schema.

# Deliverables

- `shared/events/events.go`: Standardized event definitions.
- `internal/events/events.go`: A publisher that uses the standardized events.
- Examples of how to publish and consume standardized events.
