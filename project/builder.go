package project

type BuildContext struct {
}

type Builder interface {
	Build(module *Module, ctx BuildContext) error
}
