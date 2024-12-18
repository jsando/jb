package project

type Builder interface {
	Build(module *Module) error
	Run(module *Module, args []string) error
	ResolveDependencies(module *Module) ([]PackageDependency, error)
}
