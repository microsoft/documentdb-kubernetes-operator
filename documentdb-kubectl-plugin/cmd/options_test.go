package cmd

import (
	"testing"
	"time"
)

func TestStatusOptionsCompleteDefaults(t *testing.T) {
	t.Parallel()

	o := &statusOptions{documentDBName: "  sample ", namespace: "   "}
	if err := o.complete(); err != nil {
		t.Fatalf("complete returned error: %v", err)
	}
	if o.documentDBName != "sample" {
		t.Fatalf("expected documentDBName trimmed to 'sample', got %q", o.documentDBName)
	}
	if o.namespace != defaultDocumentDBNamespace {
		t.Fatalf("expected namespace default %q, got %q", defaultDocumentDBNamespace, o.namespace)
	}
}

func TestStatusOptionsCompleteRequiresDocument(t *testing.T) {
	t.Parallel()

	o := &statusOptions{}
	if err := o.complete(); err == nil {
		t.Fatal("expected error for missing documentDBName")
	}
}

func TestEventsOptionsCompleteDefaults(t *testing.T) {
	t.Parallel()

	o := &eventsOptions{documentDBName: " sample ", namespace: "\t"}
	if err := o.complete(); err != nil {
		t.Fatalf("complete returned error: %v", err)
	}
	if o.documentDBName != "sample" {
		t.Fatalf("expected document name trimmed, got %q", o.documentDBName)
	}
	if o.namespace != defaultDocumentDBNamespace {
		t.Fatalf("expected namespace default %q, got %q", defaultDocumentDBNamespace, o.namespace)
	}
}

func TestEventsOptionsCompleteRequiresDocument(t *testing.T) {
	t.Parallel()

	o := &eventsOptions{}
	if err := o.complete(); err == nil {
		t.Fatal("expected error for missing documentDBName")
	}
}

func TestPromoteOptionsCompleteDefaults(t *testing.T) {
	t.Parallel()

	o := &promoteOptions{
		documentDBName: " sample ",
		namespace:      "",
		targetCluster:  " target ",
		waitTimeout:    0,
		pollInterval:   0,
	}
	if err := o.complete(); err != nil {
		t.Fatalf("complete returned error: %v", err)
	}
	if o.documentDBName != "sample" {
		t.Fatalf("expected document name trimmed, got %q", o.documentDBName)
	}
	if o.namespace != defaultDocumentDBNamespace {
		t.Fatalf("expected namespace default %q, got %q", defaultDocumentDBNamespace, o.namespace)
	}
	if o.targetCluster != "target" {
		t.Fatalf("expected targetCluster trimmed, got %q", o.targetCluster)
	}
	if o.waitTimeout <= 0 {
		t.Fatalf("expected waitTimeout to be positive, got %v", o.waitTimeout)
	}
	if o.pollInterval <= 0 {
		t.Fatalf("expected pollInterval to be positive, got %v", o.pollInterval)
	}
}

func TestPromoteOptionsCompleteRequiresFields(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		opt  promoteOptions
	}{
		{
			name: "missing document",
			opt:  promoteOptions{},
		},
		{
			name: "missing target",
			opt:  promoteOptions{documentDBName: "sample"},
		},
	}

	for _, tc := range testCases {
		if err := tc.opt.complete(); err == nil {
			t.Fatalf("expected error for case %q", tc.name)
		}
	}
}

func TestPromoteOptionsCompleteTrimsContexts(t *testing.T) {
	t.Parallel()

	o := &promoteOptions{
		documentDBName: "sample",
		targetCluster:  "target",
		hubContext:     "  ctx \n",
		targetContext:  " other \t",
		waitTimeout:    time.Second,
		pollInterval:   time.Millisecond,
	}
	if err := o.complete(); err != nil {
		t.Fatalf("complete returned error: %v", err)
	}
	if o.hubContext != "ctx" {
		t.Fatalf("expected hubContext trimmed to 'ctx', got %q", o.hubContext)
	}
	if o.targetContext != "other" {
		t.Fatalf("expected targetContext trimmed to 'other', got %q", o.targetContext)
	}
}
