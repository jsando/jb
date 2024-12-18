package project

import (
	"fmt"
	"sync"
)

var javaBuilder *JavaBuilder
var createJavaBuilder sync.Once

func GetBuilder(name string) (Builder, error) {
	switch name {
	case "Java":
		createJavaBuilder.Do(func() {
			javaBuilder = NewJavaBuilder()
		})
		return javaBuilder, nil
	}
	return nil, fmt.Errorf("no builder found for '%s'", name)
}
