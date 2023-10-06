// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package aggregator

import (
	"fmt"
	"time"

	agentmodel "github.com/DataDog/agent-payload/v5/process"

	"github.com/DataDog/datadog-agent/test/fakeintake/api"
)

type ProcessPayload struct {
	agentmodel.CollectorProc
	collectedTime time.Time
}

func (p ProcessPayload) name() string {
	return p.HostName
}

func (p ProcessPayload) GetTags() []string {
	return p.Host.AllTags
}

func (p ProcessPayload) GetCollectedTime() time.Time {
	return p.collectedTime
}

func ParseProcessPayload(payload api.Payload) (metrics []*ProcessPayload, err error) {
	msg, err := agentmodel.DecodeMessage(payload.Data)
	if err != nil {
		return nil, err
	}

	switch m := msg.Body.(type) {
	case *agentmodel.CollectorProc:
		return []*ProcessPayload{{CollectorProc: *m, collectedTime: payload.Timestamp}}, nil
	default:
		return nil, fmt.Errorf("unexpected type %s", msg.Header.Type)
	}
}

type ProcessAggregator struct {
	Aggregator[*ProcessPayload]
}

func NewProcessAggregator() ProcessAggregator {
	return ProcessAggregator{
		Aggregator: newAggregator(ParseProcessPayload),
	}
}
