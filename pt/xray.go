package pt

import (
	"github.com/aws/aws-xray-sdk-go/strategy/sampling"
	"github.com/aws/aws-xray-sdk-go/xray"
)

func DisableXRay() {
	_ = xray.Configure(xray.Config{
		SamplingStrategy: xrayStrategy(false),
	})
}

type xrayStrategy bool

func (sample xrayStrategy) ShouldTrace(request *sampling.Request) *sampling.Decision {
	return &sampling.Decision{
		Sample: bool(sample),
	}
}
