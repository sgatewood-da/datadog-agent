package propagation

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

func getMapFromCarrier(tm tracer.TextMapReader) map[string]string {
	if tm == nil {
		return nil
	}
	m := map[string]string{}
	tm.ForeachKey(func(key, val string) error {
		m[key] = val
		return nil
	})
	return m
}

func TestSQSMessageAttrCarrier(t *testing.T) {
	testcases := []struct {
		name     string
		attr     events.SQSMessageAttribute
		expMap   map[string]string
		expNoErr bool
	}{
		{
			name: "datadog-map",
			attr: events.SQSMessageAttribute{
				DataType: "String",
				StringValue: aws.String(`{
					"x-datadog-trace-id": "3754030949214830614",
					"x-datadog-parent-id": "9807017789787771839",
					"x-datadog-sampling-priority": "1",
					"x-datadog-tags": "_dd.p.dm=-0",
					"traceparent": "00-00000000000000003418ff4233c5c016-881986b8523c93bf-01",
					"tracestate": "dd=s:1;t.dm:-0"
				}`),
			},
			expMap: map[string]string{
				"x-datadog-trace-id":          "3754030949214830614",
				"x-datadog-parent-id":         "9807017789787771839",
				"x-datadog-sampling-priority": "1",
				"x-datadog-tags":              "_dd.p.dm=-0",
				"traceparent":                 "00-00000000000000003418ff4233c5c016-881986b8523c93bf-01",
				"tracestate":                  "dd=s:1;t.dm:-0",
			},
			expNoErr: true,
		},
		{
			name: "empty-map",
			attr: events.SQSMessageAttribute{
				DataType:    "String",
				StringValue: aws.String("{}"),
			},
			expMap:   map[string]string{},
			expNoErr: true,
		},
		{
			name: "empty-string",
			attr: events.SQSMessageAttribute{
				DataType:    "String",
				StringValue: aws.String(""),
			},
			expMap:   nil,
			expNoErr: false,
		},
		{
			name: "nil-string",
			attr: events.SQSMessageAttribute{
				DataType:    "String",
				StringValue: nil,
			},
			expMap:   nil,
			expNoErr: false,
		},
		{
			name: "wrong-data-type",
			attr: events.SQSMessageAttribute{
				DataType: "Binary",
			},
			expMap:   nil,
			expNoErr: false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			tm, err := sqsMessageAttrCarrier(tc.attr)
			t.Logf("sqsMessageAttrCarrier returned TextMapReader=%#v error=%#v", tm, err)
			assert.Equal(t, tc.expNoErr, err == nil)
			assert.Equal(t, tc.expMap, getMapFromCarrier(tm))
		})
	}
}

func TestSnsSqsMessageCarrier(t *testing.T) {
	testcases := []struct {
		name   string
		event  events.SQSMessage
		expMap map[string]string
		expErr error
	}{
		{
			name: "empty-string-body",
			event: events.SQSMessage{
				Body: "",
			},
			expMap: nil,
			expErr: errors.New("Error unmarshaling message body: unexpected end of JSON input"),
		},
		{
			name: "empty-map-body",
			event: events.SQSMessage{
				Body: "{}",
			},
			expMap: nil,
			expErr: errors.New("No Datadog trace context found"),
		},
		{
			name: "no-msg-attrs",
			event: events.SQSMessage{
				Body: `{
					"MessageAttributes": {}
				}`,
			},
			expMap: nil,
			expErr: errors.New("No Datadog trace context found"),
		},
		{
			name: "wrong-type-msg-attrs",
			event: events.SQSMessage{
				Body: `{
					"MessageAttributes": "attrs"
				}`,
			},
			expMap: nil,
			expErr: errors.New("Error unmarshaling message body: json: cannot unmarshal string into Go struct field .MessageAttributes of type map[string]struct { Type string; Value string }"),
		},
		{
			name: "non-binary-type",
			event: events.SQSMessage{
				Body: `{
					"MessageAttributes": {
						"_datadog": {
							"Type": "String",
							"Value": "Value"
						}
					}
				}`,
			},
			expMap: nil,
			expErr: errors.New("Unsupported DataType in _datadog payload"),
		},
		{
			name: "cannot-decode",
			event: events.SQSMessage{
				Body: `{
					"MessageAttributes": {
						"_datadog": {
							"Type": "Binary",
							"Value": "Value"
						}
					}
				}`,
			},
			expMap: nil,
			expErr: errors.New("Error decoding binary: illegal base64 data at input byte 4"),
		},
		{
			name: "empty-string-encoded",
			event: events.SQSMessage{
				Body: `{
					"MessageAttributes": {
						"_datadog": {
							"Type": "Binary",
							"Value": "` + base64.StdEncoding.EncodeToString([]byte(``)) + `"
						}
					}
				}`,
			},
			expMap: nil,
			expErr: errors.New("Error unmarshaling the decoded binary: unexpected end of JSON input"),
		},
		{
			name: "empty-map-encoded",
			event: events.SQSMessage{
				Body: `{
					"MessageAttributes": {
						"_datadog": {
							"Type": "Binary",
							"Value": "` + base64.StdEncoding.EncodeToString([]byte(`{}`)) + `"
						}
					}
				}`,
			},
			expMap: map[string]string{},
			expErr: nil,
		},
		{
			name: "datadog-map",
			event: events.SQSMessage{
				Body: `{
					"MessageAttributes": {
						"_datadog": {
							"Type": "Binary",
							"Value": "` + base64.StdEncoding.EncodeToString([]byte(`{
								"x-datadog-trace-id": "3754030949214830614",
								"x-datadog-parent-id": "9807017789787771839",
								"x-datadog-sampling-priority": "1",
								"x-datadog-tags": "_dd.p.dm=-0",
								"traceparent": "00-00000000000000003418ff4233c5c016-881986b8523c93bf-01",
								"tracestate": "dd=s:1;t.dm:-0"
							}`)) + `"
						}
					}
				}`,
			},
			expMap: map[string]string{
				"x-datadog-trace-id":          "3754030949214830614",
				"x-datadog-parent-id":         "9807017789787771839",
				"x-datadog-sampling-priority": "1",
				"x-datadog-tags":              "_dd.p.dm=-0",
				"traceparent":                 "00-00000000000000003418ff4233c5c016-881986b8523c93bf-01",
				"tracestate":                  "dd=s:1;t.dm:-0",
			},
			expErr: nil,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			tm, err := snsSqsMessageCarrier(tc.event)
			t.Logf("snsSqsMessageCarrier returned TextMapReader=%#v error=%#v", tm, err)
			assert.Equal(t, tc.expErr == nil, err == nil)
			if err != nil {
				assert.Equal(t, tc.expErr.Error(), err.Error())
			}
			assert.Equal(t, tc.expMap, getMapFromCarrier(tm))
		})
	}
}

func TestExtractTraceContextfromAWSTraceHeader(t *testing.T) {
	ctx := func(trace, parent uint64) *TraceContext {
		return &TraceContext{
			TraceID:  trace,
			ParentID: parent,
		}
	}

	testcases := []struct {
		name     string
		value    string
		expTc    *TraceContext
		expNoErr bool
	}{
		{
			name:     "empty string",
			value:    "",
			expTc:    nil,
			expNoErr: false,
		},
		{
			name:     "root but no parent",
			value:    "Root=1-00000000-000000000000000000000001",
			expTc:    nil,
			expNoErr: false,
		},
		{
			name:     "parent but no root",
			value:    "Parent=0000000000000001",
			expTc:    nil,
			expNoErr: false,
		},
		{
			name:     "just root and parent",
			value:    "Root=1-00000000-000000000000000000000001;Parent=0000000000000001",
			expTc:    ctx(1, 1),
			expNoErr: true,
		},
		{
			name:     "trailing semi-colon",
			value:    "Root=1-00000000-000000000000000000000001;Parent=0000000000000001;",
			expTc:    ctx(1, 1),
			expNoErr: true,
		},
		{
			name:     "trailing semi-colons",
			value:    "Root=1-00000000-000000000000000000000001;Parent=0000000000000001;;;",
			expTc:    ctx(1, 1),
			expNoErr: true,
		},
		{
			name:     "parent first",
			value:    "Parent=0000000000000009;Root=1-00000000-000000000000000000000001",
			expTc:    ctx(1, 9),
			expNoErr: true,
		},
		{
			name:     "two roots",
			value:    "Root=1-00000000-000000000000000000000005;Parent=0000000000000009;Root=1-00000000-000000000000000000000001",
			expTc:    ctx(5, 9),
			expNoErr: true,
		},
		{
			name:     "two parents",
			value:    "Root=1-00000000-000000000000000000000001;Parent=0000000000000009;Parent=0000000000000000",
			expTc:    ctx(1, 9),
			expNoErr: true,
		},
		{
			name:     "sampled is ignored",
			value:    "Root=1-00000000-000000000000000000000001;Parent=0000000000000002;Sampled=1",
			expTc:    ctx(1, 2),
			expNoErr: true,
		},
		{
			name:     "with lineage",
			value:    "Root=1-00000000-000000000000000000000001;Parent=0000000000000001;Lineage=a87bd80c:1|68fd508a:5|c512fbe3:2",
			expTc:    ctx(1, 1),
			expNoErr: true,
		},
		{
			name:     "root too long",
			value:    "Root=1-00000000-0000000000000000000000010000;Parent=0000000000000001",
			expTc:    ctx(65536, 1),
			expNoErr: true,
		},
		{
			name:     "parent too long",
			value:    "Root=1-00000000-000000000000000000000001;Parent=00000000000000010000",
			expTc:    ctx(1, 65536),
			expNoErr: true,
		},
		{
			name:     "invalid root chars",
			value:    "Root=1-00000000-00000000000000000traceID;Parent=0000000000000000",
			expTc:    nil,
			expNoErr: false,
		},
		{
			name:     "invalid parent chars",
			value:    "Root=1-00000000-000000000000000000000000;Parent=0000000000spanID",
			expTc:    nil,
			expNoErr: false,
		},
		{
			name:     "invalid root and parent chars",
			value:    "Root=1-00000000-00000000000000000traceID;Parent=0000000000spanID",
			expTc:    nil,
			expNoErr: false,
		},
		{
			name:     "large trace-id",
			value:    "Root=1-5759e988-bd862e3fe1be46a994272793;Parent=53995c3f42cd8ad8",
			expTc:    nil,
			expNoErr: false,
		},
		{
			name:     "non-zero epoch",
			value:    "Root=1-5759e988-00000000e1be46a994272793;Parent=53995c3f42cd8ad8",
			expTc:    ctx(16266516598257821587, 6023947403358210776),
			expNoErr: true,
		},
		{
			name:     "unknown key/value",
			value:    "Root=1-00000000-000000000000000000000001;Parent=0000000000000001;key=value",
			expTc:    ctx(1, 1),
			expNoErr: true,
		},
		{
			name:     "key no value",
			value:    "Root=1-00000000-000000000000000000000001;Parent=0000000000000001;key=",
			expTc:    ctx(1, 1),
			expNoErr: true,
		},
		{
			name:     "value no key",
			value:    "Root=1-00000000-000000000000000000000001;Parent=0000000000000001;=value",
			expTc:    ctx(1, 1),
			expNoErr: true,
		},
		{
			name:     "extra chars suffix",
			value:    "Root=1-00000000-000000000000000000000001;Parent=0000000000000001;value",
			expTc:    ctx(1, 1),
			expNoErr: true,
		},
		{
			name:     "root key no root value",
			value:    "Root=;Parent=0000000000000001",
			expTc:    nil,
			expNoErr: false,
		},
		{
			name:     "parent key no parent value",
			value:    "Root=1-00000000-000000000000000000000001;Parent=",
			expTc:    nil,
			expNoErr: false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)
			ctx, err := extractTraceContextfromAWSTraceHeader(tc.value)
			t.Logf("extractTraceContextfromAWSTraceHeader returned TraceContext=%#v error=%#v", ctx, err)
			assert.Equal(tc.expTc, ctx)
			assert.Equal(tc.expNoErr, err == nil)
		})
	}
}
