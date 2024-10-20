package project

const ModuleFilename = ".jbm"
const ProjectFilename = ".jbp"

type Project struct {
	modules map[string]*Module
}
