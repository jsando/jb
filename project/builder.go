package project

type BuildContext struct {
}

type Builder interface {
	Build(module *Module, ctx BuildContext) error
	Run(module *Module, args []string) error
}
