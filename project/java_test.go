package project

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJavaBuilder_Build(t *testing.T) {
	tests := []struct {
		path          string
		expectedError bool
	}{
		{
			path:          "../tests/nodeps",
			expectedError: false,
		},
		{
			path:          "../tests/refs/main",
			expectedError: false,
		},
		{
			path:          "../tests/simpledeps",
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			module, err := loadModule(tt.path)
			assert.NoError(t, err)
			err = module.Clean()
			assert.NoError(t, err)
			err = module.Build()
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func loadModule(path string) (*Module, error) {
	path, err := filepath.Abs(path)
	loader := NewModuleLoader()
	module, err := loader.GetModule(path)
	if err != nil {
		return nil, err
	}
	return module, nil
}
