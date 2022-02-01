// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

// Package noop implements a parser that simply returns its input unchanged.
package noop

import (
	"github.com/DataDog/datadog-agent/pkg/logs/internal/parsers"
	"github.com/DataDog/datadog-agent/pkg/logs/internal/parsers/internal/base"
)

// New creates a default parser that simply returns lines unchanged as messages
func New() parsers.Parser {
	p := &noop{}
	p.ParserBase.Process = p.Process
	return p
}

type noop struct {
	base.ParserBase
}

// Parse implements Parser#Parse
func (p *noop) Process(msg []byte) (parsers.Message, error) {
	return parsers.Message{Content: msg}, nil
}
