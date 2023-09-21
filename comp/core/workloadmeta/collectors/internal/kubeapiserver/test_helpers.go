// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build kubeapiserver && test

package kubeapiserver

import (
	"context"
	"testing"
	"time"

	"github.com/DataDog/datadog-agent/comp/core"
	"github.com/DataDog/datadog-agent/comp/core/config"
	"github.com/DataDog/datadog-agent/comp/core/workloadmeta"
	"github.com/DataDog/datadog-agent/pkg/util/fxutil"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"
	"k8s.io/client-go/kubernetes/fake"
)

const dummySubscriber = "dummy-subscriber"

func testCollectEvent(t *testing.T, createResource func(*fake.Clientset) error, newStore storeGenerator, expected []workloadmeta.EventBundle) {
	// Create a fake client to mock API calls.
	client := fake.NewSimpleClientset()

	overrides := map[string]interface{}{
		"cluster_agent.collect_kubernetes_tags": true,
		"language_detection.enabled":            true,
	}

	wlm := fxutil.Test[workloadmeta.Mock](t, fx.Options(
		core.MockBundle,
		fx.Replace(config.MockParams{Overrides: overrides}),
		fx.Supply(context.Background()),
		fx.Supply(workloadmeta.NewParams()),
		// GetFxOptions(),
		workloadmeta.MockModuleV2,
	))
	wlm.Start(context.Background())

	store, _ := newStore(context.TODO(), wlm, client)
	stopStore := make(chan struct{})
	go store.Run(stopStore)
	time.Sleep(5 * time.Second)

	time.Sleep(5 * time.Second)

	ch := wlm.Subscribe(dummySubscriber, workloadmeta.NormalPriority, nil)
	// When Subscribe is called, the first Bundle contains events about the items currently in the store.
	// In that case, the first bundle is empty.
	<-ch

	// Creating a resource
	err := createResource(client)
	assert.NoError(t, err)

	// Retrieving the resource in an event bundle
	bundle := <-ch
	if bundle.Ch != nil {
		close(bundle.Ch)
	}

	// nil the bundle's Ch so we can
	// deep-equal just the events later
	bundle.Ch = nil
	actual := []workloadmeta.EventBundle{bundle}
	close(stopStore)
	wlm.Unsubscribe(ch)
	assert.Equal(t, expected, actual)
}
