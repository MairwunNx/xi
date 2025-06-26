package platform

func BoolPtr(b bool) *bool {
	return &b
}

func BoolValue(b *bool, defaultValue bool) bool {
	if b == nil {
		return defaultValue
	}
	return *b
}