package utils

// https://code.google.com/p/go/issues/detail?id=6528
type RawMessage []byte

func (m RawMessage) MarshalJSON() ([]byte, error) {
	return []byte(m), nil
}
func (m *RawMessage) UnmarshalJSON(data []byte) error {
	*m = data
	return nil
}
