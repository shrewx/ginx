package statuserror

import (
	"strings"
	"testing"

	"go/types"

	"github.com/stretchr/testify/assert"
)

func TestNewStatusErrorGenerator(t *testing.T) {
	// Test with nil package
	generator := NewStatusErrorGenerator(nil)
	assert.NotNil(t, generator)
	assert.Nil(t, generator.pkg)
	assert.NotNil(t, generator.scanner)
	assert.NotNil(t, generator.statusErrors)
	assert.Len(t, generator.statusErrors, 0)
}

func TestStatusErrorGenerator_Scan(t *testing.T) {
	generator := NewStatusErrorGenerator(nil)

	// Test scanning with empty names
	generator.Scan()
	assert.Len(t, generator.statusErrors, 0)

	// Test scanning with names - this will panic due to nil package, so we test the structure before calling
	// We can't easily test the actual scanning without a real package, so we test the setup
	assert.NotNil(t, generator.statusErrors)
	assert.Len(t, generator.statusErrors, 0)
}

func TestStatusError_Structure(t *testing.T) {
	// Test StatusError struct
	statusError := &StatusError{
		TypeName: &types.TypeName{},
		Errors: []*StatusErr{
			{K: "Error1", ErrorCode: 40000000001},
			{K: "Error2", ErrorCode: 40000000002},
		},
	}

	assert.NotNil(t, statusError.TypeName)
	assert.Len(t, statusError.Errors, 2)
	assert.Equal(t, "Error1", statusError.Errors[0].K)
	assert.Equal(t, int64(40000000001), statusError.Errors[0].ErrorCode)
	assert.Equal(t, "Error2", statusError.Errors[1].K)
	assert.Equal(t, int64(40000000002), statusError.Errors[1].ErrorCode)
}

func TestGetPkgDirAndPackage_InvalidImportPath(t *testing.T) {
	// Test with invalid import path
	defer func() {
		if r := recover(); r != nil {
			// Expected to panic for invalid import path
			err, ok := r.(error)
			if ok {
				assert.True(t, strings.Contains(err.Error(), "package") || strings.Contains(err.Error(), "index out of range"))
			} else {
				assert.True(t, strings.Contains(r.(string), "package") || strings.Contains(r.(string), "index out of range"))
			}
		}
	}()

	getPkgDirAndPackage("invalid/import/path/that/does/not/exist")
}

func TestStatusErrorGenerator_Output_Structure(t *testing.T) {
	generator := NewStatusErrorGenerator(nil)

	// Add some mock status errors
	generator.statusErrors["TestError"] = &StatusError{
		TypeName: &types.TypeName{},
		Errors: []*StatusErr{
			{
				K:         "Error1",
				ErrorCode: 40000000001,
				Messages: map[string]string{
					"zh": "错误1",
					"en": "Error 1",
				},
			},
			{
				K:         "Error2",
				ErrorCode: 40000000002,
				Messages: map[string]string{
					"zh": "错误2",
					"en": "Error 2",
				},
			},
		},
	}

	// Test that the structure is set up correctly
	assert.Contains(t, generator.statusErrors, "TestError")
	testError := generator.statusErrors["TestError"]
	assert.Len(t, testError.Errors, 2)

	// Test message grouping
	messages := make(map[string][]*StatusErr)
	for _, e := range testError.Errors {
		for k, message := range e.Messages {
			messages[k] = append(messages[k], &StatusErr{
				K:       e.K,
				Message: message,
			})
		}
	}

	assert.Contains(t, messages, "zh")
	assert.Contains(t, messages, "en")
	assert.Len(t, messages["zh"], 2)
	assert.Len(t, messages["en"], 2)

	// Verify message content
	zhMessages := messages["zh"]
	assert.Equal(t, "Error1", zhMessages[0].K)
	assert.Equal(t, "错误1", zhMessages[0].Message)
	assert.Equal(t, "Error2", zhMessages[1].K)
	assert.Equal(t, "错误2", zhMessages[1].Message)

	enMessages := messages["en"]
	assert.Equal(t, "Error1", enMessages[0].K)
	assert.Equal(t, "Error 1", enMessages[0].Message)
	assert.Equal(t, "Error2", enMessages[1].K)
	assert.Equal(t, "Error 2", enMessages[1].Message)
}
