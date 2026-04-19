package tyke

import (
	"github.com/tyke/tyke/tyke/common"
	"github.com/tyke/tyke/tyke/component"
	"github.com/tyke/tyke/tyke/controller"
	"github.com/tyke/tyke/tyke/core"
	"github.com/tyke/tyke/tyke/filter"
	"github.com/tyke/tyke/tyke/ipc"
)

type (
	ContentType    = common.ContentType
	MessageType    = common.MessageType
	ProtocolHeader = common.ProtocolHeader
	JsonValue      = common.JsonValue
	BoolResult     = common.BoolResult
	ByteVecResult  = common.ByteVecResult

	ThreadPool = component.ThreadPool

	ControllerBase     = controller.ControllerBase
	RequestController  = controller.RequestController
	ResponseController = controller.ResponseController

	TykeRequest      = core.TykeRequest
	TykeResponse     = core.TykeResponse
	ResponseFuture   = core.ResponseFuture
	MetadataBase     = core.MetadataBase
	RequestMetadata  = core.RequestMetadata
	ResponseMetadata = core.ResponseMetadata
	RequestFilter    = core.RequestFilter
	ResponseFilter   = core.ResponseFilter
	TykeFramework    = core.TykeFramework
	SendDataHandler  = core.SendDataHandler

	RequestFilterAlias  = filter.RequestFilter
	ResponseFilterAlias = filter.ResponseFilter

	ClientId  = ipc.ClientId
	IpcServer = ipc.IpcServer
)

var (
	ContentTypeMap    = common.ContentTypeMap
	ProtocolMagic     = common.ProtocolMagic
	GenerateUUID      = common.GenerateUUID
	GenerateTimestamp = common.GenerateTimestamp
	IsValidUUID       = common.IsValidUUID
	GetTempDir        = common.GetTempDir

	LogDebug = common.LogDebug
	LogInfo  = common.LogInfo
	LogWarn  = common.LogWarn
	LogError = common.LogError

	OkBool     = common.OkBool
	ErrBool    = common.ErrBool
	OkByteVec  = common.OkByteVec
	ErrByteVec = common.ErrByteVec

	GetThreadPoolInstance = component.GetThreadPoolInstance
	RegisterController    = controller.RegisterController

	App                       = core.App
	AcquireRequest            = core.AcquireRequest
	ReleaseRequest            = core.ReleaseRequest
	AcquireResponse           = core.AcquireResponse
	ReleaseResponse           = core.ReleaseResponse
	GetRequestRouterInstance  = core.GetRequestRouterInstance
	GetResponseRouterInstance = core.GetResponseRouterInstance
	GetRequestRouter          = core.GetRequestRouter
	GetResponseRouter         = core.GetResponseRouter
	DispatchRequest           = core.DispatchRequest
	DispatchResponse          = core.DispatchResponse
)

const (
	DefaultTimeoutMs      = common.DefaultTimeoutMs
	DefaultBufferSize     = common.DefaultBufferSize
	DefaultThreadPoolSize = common.DefaultThreadPoolSize
	ProtocolHeaderSize    = common.ProtocolHeaderSize
	AesGcmIvLen           = common.AesGcmIvLen
	AesGcmTagLen          = common.AesGcmTagLen
	Aes256KeyLen          = common.Aes256KeyLen
	DefaultStubTimeoutMs  = common.DefaultStubTimeoutMs
	HttpStatusNotFound    = common.HttpStatusNotFound
	HttpStatusTimeout     = common.HttpStatusTimeout

	ContentTypeText   = common.ContentTypeText
	ContentTypeJson   = common.ContentTypeJson
	ContentTypeBinary = common.ContentTypeBinary

	MessageTypeNone                = common.MessageTypeNone
	MessageTypeRequest             = common.MessageTypeRequest
	MessageTypeRequestAsync        = common.MessageTypeRequestAsync
	MessageTypeRequestAsyncFunc    = common.MessageTypeRequestAsyncFunc
	MessageTypeRequestAsyncFuture  = common.MessageTypeRequestAsyncFuture
	MessageTypeResponse            = common.MessageTypeResponse
	MessageTypeResponseAsync       = common.MessageTypeResponseAsync
	MessageTypeResponseAsyncFunc   = common.MessageTypeResponseAsyncFunc
	MessageTypeResponseAsyncFuture = common.MessageTypeResponseAsyncFuture
)
