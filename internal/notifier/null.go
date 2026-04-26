package notifier

// NullNotifier is a notifier that does nothing.
type NullNotifier struct{}

// NewNullNotifier creates a new NullNotifier.
func NewNullNotifier() *NullNotifier {
	return &NullNotifier{}
}

// Notify does nothing.
func (n *NullNotifier) Notify(message string) {
	// Do nothing
}
