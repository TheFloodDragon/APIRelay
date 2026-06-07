package adaptor

import (
	"github.com/TheFloodDragon/APIRelay/internal/relay/adaptor/anthropic"
	"github.com/TheFloodDragon/APIRelay/internal/relay/adaptor/gemini"
	"github.com/TheFloodDragon/APIRelay/internal/relay/adaptor/openai"
	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
)

// GetAdaptor 根据 APIType 返回对应协议适配器。
func GetAdaptor(apiType constant.APIType) Adaptor {
	switch apiType {
	case constant.APITypeAnthropic:
		return anthropic.NewAdaptor()
	case constant.APITypeGemini:
		return gemini.NewAdaptor()
	case constant.APITypeOpenAI:
		fallthrough
	default:
		return openai.NewAdaptor()
	}
}
