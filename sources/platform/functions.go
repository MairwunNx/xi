package platform

func Curry[T any](constructor func() T, configurator func(T)) T {
	instance := constructor()
	configurator(instance)
	return instance
}

func GetGradeEmoji(grade UserGrade) string {
	switch grade {
	case GradeGold:
		return "💎"
	case GradeSilver:
		return "🥈"
	case GradeBronze:
		return "🥉"
	default:
		return "❓"
	}
}

func GetGradeNameRu(grade UserGrade) string {
	switch grade {
	case GradeGold:
		return "Золотой"
	case GradeSilver:
		return "Серебряный"
	case GradeBronze:
		return "Бронзовый"
	default:
		return "Неизвестный"
	}
}