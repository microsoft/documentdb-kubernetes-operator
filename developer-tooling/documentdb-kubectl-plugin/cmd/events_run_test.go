package cmd

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func TestEventsRunPrintsExistingEvents(t *testing.T) {
	t.Parallel()

	prevLoad := loadConfigFunc
	prevKube := kubernetesClientForConfig
	defer func() {
		loadConfigFunc = prevLoad
		kubernetesClientForConfig = prevKube
	}()

	namespace := defaultDocumentDBNamespace
	docName := "documentdb-sample"

	evt := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "documentdb-event",
			Namespace: namespace,
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "DocumentDB",
			Name:      docName,
			Namespace: namespace,
		},
		Message: "Document promoted",
		Type:    corev1.EventTypeNormal,
	}
	evt.EventTime = metav1.MicroTime{Time: time.Now()}
	evt.FirstTimestamp = metav1.NewTime(time.Now())
	evt.LastTimestamp = metav1.NewTime(time.Now())

	kubeClient := kubefake.NewSimpleClientset(evt)

	loadConfigFunc = func(string) (*rest.Config, string, error) {
		return &rest.Config{Host: "events"}, "events-context", nil
	}

	kubernetesClientForConfig = func(cfg *rest.Config) (kubernetes.Interface, error) {
		if cfg.Host != "events" {
			return nil, fmt.Errorf("unexpected host %s", cfg.Host)
		}
		return kubeClient, nil
	}

	cmd := &cobra.Command{}
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	opts := &eventsOptions{
		documentDBName: docName,
		namespace:      namespace,
		follow:         false,
	}

	if err := opts.run(context.Background(), cmd); err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %s", stderr.String())
	}

	output := stdout.String()

	checks := []string{
		"Watching events for DocumentDB",
		docName,
		"Document promoted",
	}

	for _, expected := range checks {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q, got: %s", expected, output)
		}
	}
}
