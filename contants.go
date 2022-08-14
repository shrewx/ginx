package ginx

const (
	QUERY      = "query"
	PATH       = "path"
	FORM       = "form"
	URLENCODED = "urlencoded"
	MULTIPART  = "multipart"
	BODY       = "body"
	HEADER     = "header"
	COOKIES    = "cookies"
)

const (
	I18nZH = "zh"
	I18nEN = "en"
)

const (
	EnvTag        = "env"
	DefaultConfig = "config.yml"
)

const OperationName = "x-operation-name"

// https://developer.mozilla.org/en-US/docs/Web/HTTP/Basics_of_HTTP/MIME_types
const (
	MineApplicationJson        = "application/json"
	MineApplicationOctetStream = "application/octet-stream"
	MineApplicationOgg         = "application/ogg"
	MineApplicationXML         = "application/xml"
	MineApplicationUrlencoded  = "application/x-www-form-urlencoded"
	MineMultipartForm          = "multipart/form-data"
	MineApplicationProtobuf    = "application/x-protobuf"
	MineApplicationMSGPackX    = "application/x-msgpack"
	MineApplicationMSGPack     = "application/msgpack"
	MineApplicationYaml        = "application/x-yaml"
	MineApplicationToml        = "application/toml"
	MineAudioMidi              = "audio/midi"
	MineAudioMpeg              = "audio/mpeg"
	MineAudioOgg               = "audio/ogg"
	MineAudioWave              = "audio/wav"
	MineImageBmp               = "image/bmp"
	MineImageGif               = "image/gif"
	MineImageJpeg              = "image/jpeg"
	MineImagePng               = "image/png"
	MineImageSvg               = "image/svg+xml"
	MineImageWebp              = "image/webp"
	MineTextCss                = "text/css"
	MineTextHtml               = "text/html"
	MineTextXml                = "text/xml"
	MineTextPlain              = "text/plain"
	MineVideoOgg               = "video/ogg"
	MineVideoWebm              = "video/webm"
)
