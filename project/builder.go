package project

type Builder interface {
	Clean(module *Module) error
	Build(module *Module) error
	Run(module *Module, args []string) error
	ResolveDependencies(module *Module) ([]PackageDependency, error)
	Publish(m *Module, repoURL, user, password string) error
}
