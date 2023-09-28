package propagation

import (
	"encoding/base64"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
)

type uintItem struct {
	asUint uint64
	asStr  string
}
type intItem struct {
	asInt int
	asStr string
}
type context struct {
	trace    uintItem
	span     uintItem
	priority intItem
}

var (
	dd = context{
		trace:    uintItem{1, "0000000000000000001"},
		span:     uintItem{2, "0000000000000000002"},
		priority: intItem{2, "2"},
	}
	w3c = context{
		trace:    uintItem{3, "0000000000000003"},
		span:     uintItem{4, "0000000000000004"},
		priority: intItem{3, "3"},
	}
	ddx = context{
		trace:    uintItem{5, "0000000000000000005"},
		span:     uintItem{6, "0000000000000000006"},
		priority: intItem{0, "0"},
	}
	x = context{
		trace:    uintItem{7, "0000000000000000007"},
		span:     uintItem{8, "0000000000000000008"},
		priority: intItem{0, "0"},
	}
)

var (
	headersNone  = ""
	headersEmpty = "{}"
	headersAll   = `{
		"x-datadog-trace-id": "` + dd.trace.asStr + `",
		"x-datadog-parent-id": "` + dd.span.asStr + `",
		"x-datadog-sampling-priority": "` + dd.priority.asStr + `",
		"x-datadog-tags": "_dd.p.dm=-0",
		"traceparent": "00-0000000000000000` + w3c.trace.asStr + "-" + w3c.span.asStr + `-01",
		"tracestate": "dd=s:` + w3c.priority.asStr + `;t.dm:-0"
	}`
	headersDD = `{
		"x-datadog-trace-id": "` + dd.trace.asStr + `",
		"x-datadog-parent-id": "` + dd.span.asStr + `",
		"x-datadog-sampling-priority": "` + dd.priority.asStr + `",
		"x-datadog-tags": "_dd.p.dm=-0"
	}`
	headersW3C = `{
		"traceparent": "00-0000000000000000` + w3c.trace.asStr + "-" + w3c.span.asStr + `-01",
		"tracestate": "dd=s:` + w3c.priority.asStr + `;t.dm:-0"
	}`
	headersDdXray = "Root=1-00000000-00000000" + ddx.trace.asStr + ";Parent=" + ddx.span.asStr
	headersXray   = "Root=1-12345678-12345678" + x.trace.asStr + ";Parent=" + x.span.asStr

	eventSqsMessage = func(sqsHdrs, snsHdrs, awsHdr string) events.SQSMessage {
		e := events.SQSMessage{}
		if sqsHdrs != "" {
			e.MessageAttributes = map[string]events.SQSMessageAttribute{
				"_datadog": events.SQSMessageAttribute{
					DataType:    "String",
					StringValue: aws.String(sqsHdrs),
				},
			}
		}
		if snsHdrs != "" {
			e.Body = `{
				"MessageAttributes": {
					"_datadog": {
						"Type": "Binary",
						"Value": "` + base64.StdEncoding.EncodeToString([]byte(snsHdrs)) + `"
					}
				}
			}`
		}
		if awsHdr != "" {
			e.Attributes = map[string]string{
				awsTraceHeader: awsHdr,
			}
		}
		return e
	}
)

func TestNilExtractor(t *testing.T) {
	var extractor *Extractor
	tc, err := extractor.Extract("hello world")
	t.Logf("Extract returned TraceContext=%#v error=%#v", tc, err)
	assert.Equal(t, "Extraction not configured", err.Error())
	assert.Nil(t, tc)
}

func TestExtractorExtract(t *testing.T) {
	testcases := []struct {
		name     string
		event    interface{}
		expCtx   *TraceContext
		expNoErr bool
	}{
		{
			name:     "unsupported-event",
			event:    "hello world",
			expCtx:   nil,
			expNoErr: false,
		},
		{
			name: "unable-to-get-carrier",
			event: events.SQSMessage{
				Body: "",
			},
			expCtx:   nil,
			expNoErr: false,
		},
		{
			name:     "extraction-error",
			event:    eventSqsMessage(headersEmpty, headersNone, headersNone),
			expCtx:   nil,
			expNoErr: false,
		},
		{
			name:  "extract-from-sqs",
			event: eventSqsMessage(headersAll, headersNone, headersNone),
			expCtx: &TraceContext{
				TraceID:  w3c.trace.asUint,
				ParentID: w3c.span.asUint,
				Priority: w3c.priority.asInt,
			},
			expNoErr: true,
		},
		{
			name:  "extract-from-snssqs",
			event: eventSqsMessage(headersNone, headersAll, headersNone),
			expCtx: &TraceContext{
				TraceID:  w3c.trace.asUint,
				ParentID: w3c.span.asUint,
				Priority: w3c.priority.asInt,
			},
			expNoErr: true,
		},
		{
			name:  "extract-from-sqs-attrs",
			event: eventSqsMessage(headersW3C, headersDD, headersDdXray),
			expCtx: &TraceContext{
				TraceID:  ddx.trace.asUint,
				ParentID: ddx.span.asUint,
				Priority: ddx.priority.asInt,
			},
			expNoErr: true,
		},
		{
			name:  "sqs-precidence-attrs",
			event: eventSqsMessage(headersW3C, headersDD, headersDdXray),
			expCtx: &TraceContext{
				TraceID:  ddx.trace.asUint,
				ParentID: ddx.span.asUint,
				Priority: ddx.priority.asInt,
			},
			expNoErr: true,
		},
		{
			name:  "sqs-precidence-sqs",
			event: eventSqsMessage(headersW3C, headersDD, headersXray),
			expCtx: &TraceContext{
				TraceID:  w3c.trace.asUint,
				ParentID: w3c.span.asUint,
				Priority: w3c.priority.asInt,
			},
			expNoErr: true,
		},
		{
			name:  "sqs-precidence-snssqs",
			event: eventSqsMessage(headersNone, headersDD, headersXray),
			expCtx: &TraceContext{
				TraceID:  dd.trace.asUint,
				ParentID: dd.span.asUint,
				Priority: dd.priority.asInt,
			},
			expNoErr: true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			extractor := NewExtractor()
			ctx, err := extractor.Extract(tc.event)
			t.Logf("Extract returned TraceContext=%#v error=%#v", ctx, err)
			assert.Equal(t, tc.expNoErr, err == nil)
			assert.Equal(t, tc.expCtx, ctx)
		})
	}
}

func TestPropagationStyle(t *testing.T) {
	testcases := []struct {
		name       string
		propType   string
		hdrs       string
		expTraceID uint64
	}{
		{
			name:       "no-type-headers-all",
			propType:   "",
			hdrs:       headersAll,
			expTraceID: w3c.trace.asUint,
		},
		{
			name:       "datadog-type-headers-all",
			propType:   "datadog",
			hdrs:       headersAll,
			expTraceID: dd.trace.asUint,
		},
		{
			name:       "tracecontet-type-headers-all",
			propType:   "tracecontext",
			hdrs:       headersAll,
			expTraceID: w3c.trace.asUint,
		},
		{
			// XXX: This is surprising
			// The go tracer is designed to always place the tracecontext propagator first
			// see https://github.com/DataDog/dd-trace-go/blob/6a938b3b4054ce036cc60147ab42a86f743fcdd5/ddtrace/tracer/textmap.go#L231
			name:       "datadog,tracecontext-type-headers-all",
			propType:   "datadog,tracecontext",
			hdrs:       headersAll,
			expTraceID: w3c.trace.asUint,
		},
		{
			name:       "tracecontext,datadog-type-headers-all",
			propType:   "tracecontext,datadog",
			hdrs:       headersAll,
			expTraceID: w3c.trace.asUint,
		},
		{
			name:       "datadog-type-headers-w3c",
			propType:   "datadog",
			hdrs:       headersW3C,
			expTraceID: 0,
		},
		{
			name:       "tracecontet-type-headers-dd",
			propType:   "tracecontext",
			hdrs:       headersDD,
			expTraceID: 0,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DD_TRACE_PROPAGATION_STYLE", tc.propType)
			extractor := NewExtractor()
			event := eventSqsMessage(tc.hdrs, headersNone, headersNone)
			ctx, err := extractor.Extract(event)
			t.Logf("Extract returned TraceContext=%#v error=%#v", ctx, err)
			if tc.expTraceID == 0 {
				assert.NotNil(t, err)
				assert.Nil(t, ctx)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, tc.expTraceID, ctx.TraceID)
			}
		})
	}
}
