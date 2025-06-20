package platform

func Curry[T any](constructor func() T, configurator func(T)) T {
	instance := constructor()
	configurator(instance)
	return instance
}