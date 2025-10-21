package ginx

import (
	"bytes"
	"github.com/shrewx/ginx/pkg/utils"

	"net/url"

	"github.com/gin-gonic/gin"
)

type Attachment struct {
	filename    string
	contentType string
	bytes.Buffer
}

func (a *Attachment) ContentType() string {
	if a.contentType == "" {
		return MineApplicationOctetStream
	}
	return a.contentType
}

func (a *Attachment) Header(ctx *gin.Context) {
	if utils.IsASCII(a.filename) {
		ctx.Writer.Header().Set("Content-Disposition", `attachment; filename="`+a.filename+`"`)
	} else {
		ctx.Writer.Header().Set("Content-Disposition", `attachment; filename*=UTF-8''`+url.QueryEscape(a.filename))
	}
}

func (a *Attachment) Bytes() []byte {
	return a.Buffer.Bytes()
}

func NewAttachment(filename string, contentType string) *Attachment {
	return &Attachment{
		filename:    filename,
		contentType: contentType,
	}
}

type ApplicationOgg struct {
	bytes.Buffer
}

func (a *ApplicationOgg) ContentType() string {
	return MineApplicationOgg
}

func (a *ApplicationOgg) Bytes() []byte { return a.Buffer.Bytes() }

func NewApplicationOgg() *ApplicationOgg {
	return &ApplicationOgg{}
}

type AudioMidi struct {
	bytes.Buffer
}

func (a *AudioMidi) ContentType() string {
	return MineAudioMidi
}

func (a *AudioMidi) Bytes() []byte { return a.Buffer.Bytes() }

func NewAudioMidi() *AudioMidi {
	return &AudioMidi{}
}

type AudioMpeg struct {
	bytes.Buffer
}

func (a *AudioMpeg) ContentType() string {
	return MineAudioMpeg
}

func (a *AudioMpeg) Bytes() []byte { return a.Buffer.Bytes() }

func NewAudioMpeg() *AudioMpeg {
	return &AudioMpeg{}
}

type AudioOgg struct {
	bytes.Buffer
}

func (a *AudioOgg) ContentType() string {
	return MineAudioOgg
}

func (a *AudioOgg) Bytes() []byte { return a.Buffer.Bytes() }

func NewAudioOgg() *AudioOgg {
	return &AudioOgg{}
}

type AudioWave struct {
	bytes.Buffer
}

func (a *AudioWave) ContentType() string {
	return MineAudioWave
}

func (a *AudioWave) Bytes() []byte { return a.Buffer.Bytes() }

func NewAudioWave() *AudioWave {
	return &AudioWave{}
}

type AudioWebm struct {
	bytes.Buffer
}

func (a *AudioWebm) ContentType() string {
	return MineAudioWebm
}

func (a *AudioWebm) Bytes() []byte { return a.Buffer.Bytes() }

func NewAudioWebm() *AudioWebm {
	return &AudioWebm{}
}

func NewImageBmp() *ImageBmp {
	return &ImageBmp{}
}

type ImageBmp struct {
	bytes.Buffer
}

func (i *ImageBmp) ContentType() string {
	return MineImageBmp
}

func (i *ImageBmp) Bytes() []byte { return i.Buffer.Bytes() }

func NewImageGIF() *ImageGIF {
	return &ImageGIF{}
}

type ImageGIF struct {
	bytes.Buffer
}

func (i *ImageGIF) ContentType() string {
	return MineImageGif
}

func (i *ImageGIF) Bytes() []byte { return i.Buffer.Bytes() }

func NewImageJPEG() *ImageJPEG {
	return &ImageJPEG{}
}

type ImageJPEG struct {
	bytes.Buffer
}

func (i *ImageJPEG) ContentType() string {
	return MineImageJpeg
}

func (i *ImageJPEG) Bytes() []byte { return i.Buffer.Bytes() }

func NewImagePNG() *ImagePNG {
	return &ImagePNG{}
}

type ImagePNG struct {
	bytes.Buffer
}

func (i *ImagePNG) ContentType() string {
	return MineImagePng
}

func (i *ImagePNG) Bytes() []byte { return i.Buffer.Bytes() }

func NewImageSVG() *ImageSVG {
	return &ImageSVG{}
}

type ImageSVG struct {
	bytes.Buffer
}

func (ImageSVG) ContentType() string {
	return MineImageSvg
}

func (i *ImageSVG) Bytes() []byte { return i.Buffer.Bytes() }

func NewImageWebp() *ImageWebp {
	return &ImageWebp{}
}

type ImageWebp struct {
	bytes.Buffer
}

func (i *ImageWebp) ContentType() string {
	return MineImageWebp
}

func (i *ImageWebp) Bytes() []byte { return i.Buffer.Bytes() }

func NewCSS() *CSS {
	return &CSS{}
}

type CSS struct {
	bytes.Buffer
}

func (c *CSS) ContentType() string {
	return MineTextCss
}

func (c *CSS) Bytes() []byte { return c.Buffer.Bytes() }

func NewHTML() *HTML {
	return &HTML{}
}

type HTML struct {
	bytes.Buffer
}

func (h *HTML) ContentType() string {
	return MineTextHtml
}

func (h *HTML) Bytes() []byte { return h.Buffer.Bytes() }

func NewPlain() *Plain {
	return &Plain{}
}

type Plain struct {
	bytes.Buffer
}

func (p *Plain) ContentType() string {
	return MineTextPlain
}

func (p *Plain) Bytes() []byte { return p.Buffer.Bytes() }

func NewVideoOgg() *VideoOgg {
	return &VideoOgg{}
}

type VideoOgg struct {
	bytes.Buffer
}

func (v *VideoOgg) ContentType() string {
	return MineVideoOgg
}
func (v *VideoOgg) Bytes() []byte { return v.Buffer.Bytes() }

func NewVideoWebm() *VideoWebm {
	return &VideoWebm{}
}

type VideoWebm struct {
	bytes.Buffer
}

func (v *VideoWebm) ContentType() string {
	return MineVideoWebm
}

func (v *VideoWebm) Bytes() []byte { return v.Buffer.Bytes() }
