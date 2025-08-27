package ginx

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestAttachment(t *testing.T) {
	// 测试基本的Attachment
	attachment := NewAttachment("test.txt", "text/plain")
	attachment.WriteString("test content")

	assert.Equal(t, "test.txt", attachment.filename)
	assert.Equal(t, "text/plain", attachment.contentType)
	assert.Equal(t, "test content", attachment.String())
	assert.Equal(t, []byte("test content"), attachment.Bytes())
	assert.Equal(t, "text/plain", attachment.ContentType())

	// 测试默认ContentType
	attachment2 := NewAttachment("test.bin", "")
	assert.Equal(t, MineApplicationOctetStream, attachment2.ContentType())
}

func TestAttachment_Header(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 测试ASCII文件名
	attachment := NewAttachment("test.txt", "text/plain")
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	attachment.Header(ctx)
	assert.Equal(t, `attachment; filename="test.txt"`, w.Header().Get("Content-Disposition"))

	// 测试非ASCII文件名
	attachment2 := NewAttachment("测试.txt", "text/plain")
	ctx2, _ := gin.CreateTestContext(w)

	attachment2.Header(ctx2)
	assert.Contains(t, w.Header().Get("Content-Disposition"), "filename*=UTF-8''")
}

func TestMineTypes(t *testing.T) {
	tests := []struct {
		name        string
		createFunc  func() interface{}
		contentType string
	}{
		{
			name:        "ApplicationOgg",
			createFunc:  func() interface{} { return NewApplicationOgg() },
			contentType: MineApplicationOgg,
		},
		{
			name:        "AudioMidi",
			createFunc:  func() interface{} { return NewAudioMidi() },
			contentType: MineAudioMidi,
		},
		{
			name:        "AudioMpeg",
			createFunc:  func() interface{} { return NewAudioMpeg() },
			contentType: MineAudioMpeg,
		},
		{
			name:        "AudioOgg",
			createFunc:  func() interface{} { return NewAudioOgg() },
			contentType: MineAudioOgg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instance := tt.createFunc()

			// 测试ContentType
			if contentTyper, ok := instance.(interface{ ContentType() string }); ok {
				assert.Equal(t, tt.contentType, contentTyper.ContentType())
			}

			// 测试Bytes
			if byter, ok := instance.(interface{ Bytes() []byte }); ok {
				// 新创建的实例Bytes()可能返回nil或空切片
				bytes := byter.Bytes()
				// 不检查是否为nil，只检查长度
				assert.Equal(t, 0, len(bytes))
			}

			// 测试Write
			if writer, ok := instance.(interface{ Write([]byte) (int, error) }); ok {
				data := []byte("test data")
				n, err := writer.Write(data)
				assert.NoError(t, err)
				assert.Equal(t, len(data), n)
			}
		})
	}
}
