package relay

import (
	"github.com/apirelay/apirelay/constant"
	"github.com/apirelay/apirelay/relay/adaptor"
	"github.com/apirelay/apirelay/relay/adaptor/openai"
)

// GetAdaptor 根据上游协议类型返回对应适配器。
func GetAdaptor(apiType constant.APIType) adaptor.Adaptor {
	switch apiType {
	case constant.APITypeOpenAI:
		return &openai.Adaptor{}
	case constant.APITypeAnthropic:
		// 阶段3 接入 anthropic 适配器
		return nil
	case constant.APITypeResponses:
		// 阶段3 接入 responses 适配器
		return nil
	default:
		return nil
	}
}
