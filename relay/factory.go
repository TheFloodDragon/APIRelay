package relay

import (
	"github.com/apirelay/apirelay/constant"
	"github.com/apirelay/apirelay/relay/adaptor"
	"github.com/apirelay/apirelay/relay/adaptor/anthropic"
	"github.com/apirelay/apirelay/relay/adaptor/openai"
	"github.com/apirelay/apirelay/relay/adaptor/responses"
)

// GetAdaptor 根据上游协议类型返回对应适配器。
func GetAdaptor(apiType constant.APIType) adaptor.Adaptor {
	switch apiType {
	case constant.APITypeOpenAI:
		return &openai.Adaptor{}
	case constant.APITypeAnthropic:
		return &anthropic.Adaptor{}
	case constant.APITypeResponses:
		return &responses.Adaptor{}
	default:
		return nil
	}
}
