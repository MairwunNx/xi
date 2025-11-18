package format

func Pluralify(count int, one, few, many string) string {
	n := count % 100
	if n >= 11 && n <= 19 {
		return many
	}
	n = count % 10
	if n == 1 {
		return one
	}
	if n >= 2 && n <= 4 {
		return few
	}
	return many
}