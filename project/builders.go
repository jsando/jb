package project

import (
	"fmt"
)

func GetBuilder(name string) (Builder, error) {
	switch name {
	case "Java":
		return &JavaBuilder{}, nil
	}
	return nil, fmt.Errorf("no builder found for '%s'", name)
}
