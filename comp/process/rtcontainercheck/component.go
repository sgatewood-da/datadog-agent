// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

// Package rtcontainercheck implements a component to handle realtime Container data collection in the Process Agent.
package rtcontainercheck

import (
	"go.uber.org/fx"

	"github.com/DataDog/datadog-agent/comp/process/types"
	"github.com/DataDog/datadog-agent/pkg/util/module/fxutil"
)

// team: processes

type Component interface {
	types.CheckComponent
}

// Module defines the fx options for this component.
var Module = fxutil.Component(
	fx.Provide(newCheck),
)
