package propagation

import (
	"errors"

	"github.com/aws/aws-lambda-go/events"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type Extractor struct {
	propagator tracer.Propagator
}

type TraceContext struct {
	TraceID  uint64
	ParentID uint64
}

func NewExtractor() Extractor {
	prop := tracer.NewPropagator(nil)
	return Extractor{
		propagator: prop,
	}
}

func (e *Extractor) Extract(event interface{}) (*TraceContext, error) {
	if e == nil {
		return nil, errors.New("Extraction not configured")
	}
	var carrier tracer.TextMapReader
	var err error
	switch ev := event.(type) {
	case events.SQSMessage:
		if attr, ok := ev.Attributes[awsTraceHeader]; ok {
			if tc, err := extractTraceContextfromAWSTraceHeader(attr); err == nil {
				// Return early if AWSTraceHeader contains trace context
				return tc, nil
			}
		}
		carrier, err = sqsMessageCarrier(ev)
	default:
		err = errors.New("Unsupported event type for trace context extraction")
	}
	if err != nil {
		return nil, err
	}
	sp, err := e.propagator.Extract(carrier)
	if err != nil {
		return nil, err
	}
	return &TraceContext{
		TraceID:  sp.TraceID(),
		ParentID: sp.SpanID(),
	}, nil
}

type kvTextMap map[string]string

func (m kvTextMap) ForeachKey(handler func(key, val string) error) error {
	for k, v := range m {
		if err := handler(k, v); err != nil {
			return err
		}
	}
	return nil
}
