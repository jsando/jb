package builders

import (
	"fmt"
	"github.com/jsando/jb/project"
)

func GetBuilder(name string) (project.Builder, error) {
	switch name {
	case "Java":
		return &JavaBuilder{}, nil
	}
	return nil, fmt.Errorf("no builder found for '%s'", name)
}
