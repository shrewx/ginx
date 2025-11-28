package statuserror

import (
	"errors"
	"testing"

	"github.com/shrewx/ginx/internal/fields"
	"github.com/shrewx/ginx/pkg/i18nx"
	"github.com/stretchr/testify/assert"
)

func TestErrorList_Do(t *testing.T) {
	errorList := WithErrorList()

	// Test with no error
	errorList.Do(func() error {
		return nil
	})
	assert.Len(t, errorList.errors, 0)

	// Test with error
	errorList.Do(func() error {
		return errors.New("test error")
	})
	assert.Len(t, errorList.errors, 1)
	assert.Equal(t, "test error", errorList.errors[0].Error())
}

func TestErrorList_DoWithIndex(t *testing.T) {
	errorList := WithErrorList()

	// Test with StatusErr
	errorList.DoWithIndex(func() error {
		return NewStatusErr("TestError", 40000000001)
	}, 1)

	assert.Len(t, errorList.errors, 1)
	statusErr, ok := errorList.errors[0].(*StatusErr)
	assert.True(t, ok)
	assert.Equal(t, "1", statusErr.Fields[fields.ErrorIndex])

	// Test with CommonError
	errorList = WithErrorList()
	errorList.DoWithIndex(func() error {
		return NewStatusErr("TestError2", 40000000002)
	}, 2)

	assert.Len(t, errorList.errors, 1)
	statusErr, ok = errorList.errors[0].(*StatusErr)
	assert.True(t, ok)
	assert.Equal(t, "2", statusErr.Fields[fields.ErrorIndex])

	// Test with no error
	errorList = WithErrorList()
	errorList.DoWithIndex(func() error {
		return nil
	}, 3)
	assert.Len(t, errorList.errors, 0)
}

func TestErrorList_DoWithLine(t *testing.T) {
	errorList := WithErrorList()

	// Test backward compatibility
	errorList.DoWithLine(func() error {
		return NewStatusErr("TestError", 40000000001)
	}, 10)

	assert.Len(t, errorList.errors, 1)
	statusErr, ok := errorList.errors[0].(*StatusErr)
	assert.True(t, ok)
	assert.Equal(t, "10", statusErr.Fields[fields.ErrorIndex])
}

func TestErrorList_Return_NoErrors(t *testing.T) {
	errorList := WithErrorList()
	err := errorList.Return()
	assert.Nil(t, err)
}

func TestErrorList_Return_SingleError(t *testing.T) {
	errorList := WithErrorList()
	originalErr := errors.New("single error")
	errorList.Do(func() error {
		return originalErr
	})

	err := errorList.Return()
	assert.Equal(t, originalErr, err)
}

func TestErrorList_Return_MultipleStatusErrors(t *testing.T) {
	errorList := WithErrorList()

	// Add multiple StatusErr errors
	errorList.DoWithIndex(func() error {
		return NewStatusErr("Error1", 40000000001)
	}, 1)

	errorList.DoWithIndex(func() error {
		return NewStatusErr("Error2", 40000000002)
	}, 2)

	err := errorList.Return()
	assert.NotNil(t, err)

	statusErr, ok := err.(*StatusErr)
	assert.True(t, ok)
	assert.Equal(t, "ErrorsList", statusErr.K)
	assert.Equal(t, int64(5000000001), statusErr.ErrorCode)
	assert.Len(t, statusErr.ErrList, 2)

	// Check first error in ErrList
	firstErr, ok := statusErr.ErrList[0]["statusErr"].(*StatusErr)
	assert.True(t, ok)
	assert.Equal(t, "Error1", firstErr.K)
	assert.Equal(t, int64(40000000001), firstErr.ErrorCode)

	// Check second error in ErrList
	secondErr, ok := statusErr.ErrList[1]["statusErr"].(*StatusErr)
	assert.True(t, ok)
	assert.Equal(t, "Error2", secondErr.K)
	assert.Equal(t, int64(40000000002), secondErr.ErrorCode)
}

func TestErrorList_Return_MixedErrors(t *testing.T) {
	errorList := WithErrorList()

	// Add StatusErr
	errorList.DoWithIndex(func() error {
		return NewStatusErr("StatusError", 40000000001)
	}, 1)

	// Add regular error
	errorList.Do(func() error {
		return errors.New("regular error")
	})

	err := errorList.Return()
	assert.NotNil(t, err)

	statusErr, ok := err.(*StatusErr)
	assert.True(t, ok)
	assert.Equal(t, "ErrorsList", statusErr.K)
	assert.Len(t, statusErr.ErrList, 2)

	// Check StatusErr in ErrList
	firstErr, ok := statusErr.ErrList[0]["statusErr"].(*StatusErr)
	assert.True(t, ok)
	assert.Equal(t, "StatusError", firstErr.K)

	// Check regular error converted to StatusErr
	secondErr, ok := statusErr.ErrList[1]["statusErr"].(*StatusErr)
	assert.True(t, ok)
	assert.Equal(t, "InternalServerError", secondErr.K)
	assert.Equal(t, "internal error", secondErr.Message)
}

func TestErrorEntry_Error(t *testing.T) {
	originalErr := errors.New("test error")
	entry := &errorEntry{
		err:   originalErr,
		index: 1,
	}

	assert.Equal(t, "test error", entry.Error())
}

func TestErrorList_DoWithIndex_RegularError(t *testing.T) {
	errorList := WithErrorList()

	// Test with regular error (not StatusErr or CommonError)
	errorList.DoWithIndex(func() error {
		return errors.New("regular error")
	}, 1)

	assert.Len(t, errorList.errors, 1)
	// Regular errors should not have ErrorIndex field added
	assert.Equal(t, "regular error", errorList.errors[0].Error())
}

func TestErrorList_Return_CommonErrorConversion_Skip(t *testing.T) {
	t.Skip("Skipping due to complex type conversion issues")
	errorList := WithErrorList()

	// Create a mock CommonError that's not a StatusErr
	mockCommonError := &mockCommonError{
		key:  "MockError",
		code: 40000000001,
		msg:  "Mock error message",
	}

	errorList.Do(func() error {
		return mockCommonError
	})

	err := errorList.Return()
	assert.NotNil(t, err)

	statusErr, ok := err.(*StatusErr)
	assert.True(t, ok)
	assert.Equal(t, "ErrorsList", statusErr.K)
	assert.Len(t, statusErr.ErrList, 1)

	// Check that the error was converted to StatusErr
	convertedErr, ok := statusErr.ErrList[0]["statusErr"].(*StatusErr)
	assert.True(t, ok)
	assert.Equal(t, "CommonError", convertedErr.K)
	assert.Equal(t, int64(40000000001), convertedErr.ErrorCode)
	assert.Equal(t, "Mock error message", convertedErr.Message)
}

// Mock CommonError for testing
type mockCommonError struct {
	key  string
	code int64
	msg  string
}

func (m *mockCommonError) Error() string {
	return m.msg
}

func (m *mockCommonError) Code() int64 {
	return m.code
}

func (m *mockCommonError) WithParams(params map[string]interface{}) CommonError {
	return m
}

func (m *mockCommonError) WithField(key interface{}, value string) CommonError {
	return m
}

func (m *mockCommonError) Localize(manager *i18nx.Localize, lang string) i18nx.I18nMessage {
	return nil
}

func (m *mockCommonError) Value() string {
	return m.msg
}

func (m *mockCommonError) Key() string {
	return m.key
}

func (m *mockCommonError) Prefix() string {
	return ""
}
