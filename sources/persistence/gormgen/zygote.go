package main

import (
	"ximanager/sources/persistence/entities"

	"gorm.io/gen"
)

func main() {
	g := gen.NewGenerator(gen.Config{
		OutPath:      "./sources/persistence/gormdao/query",
		ModelPkgPath: "./sources/persistence/gormdao/model",
		Mode:         gen.WithDefaultQuery | gen.WithQueryInterface,
	})

	g.ApplyBasic(entities.User{}, entities.Donation{}, entities.Message{}, entities.Mode{}, entities.SelectedMode{}, entities.Pin{}, entities.Usage{})
	g.Execute()
}
