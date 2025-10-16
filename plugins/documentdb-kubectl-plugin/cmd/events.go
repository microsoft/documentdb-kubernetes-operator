package cmd

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
)

type eventsOptions struct {
	documentDBName string
	namespace      string
	kubeContext    string
	follow         bool
	since          time.Duration
}

func newEventsCommand() *cobra.Command {
	opts := &eventsOptions{namespace: "default"}

	cmd := &cobra.Command{
		Use:   "events",
		Short: "Stream events associated with a DocumentDB resource",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.complete(); err != nil {
				return err
			}
			return opts.run(cmd.Context(), cmd)
		},
	}

	cmd.Flags().StringVar(&opts.documentDBName, "documentdb", opts.documentDBName, "Name of the DocumentDB resource to inspect")
	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", opts.namespace, "Namespace containing the DocumentDB resource")
	cmd.Flags().StringVar(&opts.kubeContext, "context", opts.kubeContext, "Kubeconfig context to use (defaults to current context)")
	cmd.Flags().BoolVarP(&opts.follow, "follow", "f", true, "Stream events until interrupted")
	cmd.Flags().DurationVar(&opts.since, "since", 0, "Only show events newer than this duration (e.g. 1h); 0 shows all available")

	_ = cmd.MarkFlagRequired("documentdb")

	return cmd
}

func (o *eventsOptions) complete() error {
	if o.documentDBName == "" {
		return fmt.Errorf("--documentdb is required")
	}
	if o.namespace == "" {
		o.namespace = "default"
	}
	return nil
}

func (o *eventsOptions) run(ctx context.Context, cmd *cobra.Command) error {
	config, contextName, err := loadConfig(o.kubeContext)
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}
	if contextName == "" {
		contextName = "(current)"
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Watching events for DocumentDB %s/%s (context %s)\n", o.namespace, o.documentDBName, contextName)

	listOptions := metav1.ListOptions{
		FieldSelector: fields.AndSelectors(
			fields.OneTermEqualSelector("involvedObject.kind", "DocumentDB"),
			fields.OneTermEqualSelector("involvedObject.name", o.documentDBName),
		).String(),
	}

	evtClient := clientset.CoreV1().Events(o.namespace)
	evtList, err := evtClient.List(ctx, listOptions)
	if err != nil {
		return fmt.Errorf("failed to list events: %w", err)
	}

	filterSince := time.Time{}
	if o.since > 0 {
		filterSince = time.Now().Add(-o.since)
	}

	printedEvents := 0
	for idx := range evtList.Items {
		if !eventAfter(&evtList.Items[idx], filterSince) {
			continue
		}
		printEvent(cmd.OutOrStdout(), &evtList.Items[idx])
		printedEvents++
	}

	if !o.follow {
		if printedEvents == 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "No events found for DocumentDB %s/%s.\n", o.namespace, o.documentDBName)
		}
		return nil
	}

	if printedEvents == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No events found yet; watching for new events...")
	}

	listOptions.ResourceVersion = evtList.ResourceVersion
	watcher, err := evtClient.Watch(ctx, listOptions)
	if err != nil {
		return fmt.Errorf("failed to watch events: %w", err)
	}
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case evt, ok := <-watcher.ResultChan():
			if !ok {
				return io.EOF
			}
			k8sEvent, ok := evt.Object.(*corev1.Event)
			if !ok {
				continue
			}
			if !eventAfter(k8sEvent, filterSince) {
				continue
			}
			printEvent(cmd.OutOrStdout(), k8sEvent)
		}
	}
}

func eventAfter(evt *corev1.Event, threshold time.Time) bool {
	if threshold.IsZero() {
		return true
	}
	timestamp := mostRecentEventTime(evt)
	return timestamp.After(threshold)
}

func printEvent(out io.Writer, evt *corev1.Event) {
	timestamp := mostRecentEventTime(evt)
	fmt.Fprintf(out, "%s\t%s/%s\t%s\t%s\n",
		timestamp.Format(time.RFC3339),
		evt.InvolvedObject.Kind,
		evt.InvolvedObject.Name,
		evt.Type,
		evt.Message,
	)
}

func mostRecentEventTime(evt *corev1.Event) time.Time {
	if !evt.EventTime.IsZero() {
		return evt.EventTime.Time
	}
	if !evt.LastTimestamp.IsZero() {
		return evt.LastTimestamp.Time
	}
	if evt.Series != nil && !evt.Series.LastObservedTime.IsZero() {
		return evt.Series.LastObservedTime.Time
	}
	if !evt.CreationTimestamp.IsZero() {
		return evt.CreationTimestamp.Time
	}
	return time.Now()
}
