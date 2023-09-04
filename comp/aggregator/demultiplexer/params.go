// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package demultiplexer

import "github.com/DataDog/datadog-agent/pkg/aggregator"

type Params struct {
	Options aggregator.AgentDemultiplexerOptions
}