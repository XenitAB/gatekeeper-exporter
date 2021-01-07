package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	eventsv1beta1 "k8s.io/api/events/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	watch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	kubeconfig := flag.String("kubeconfig", os.Getenv("KUBECONFIG"), "Path to kubeconfig file")
	eventNamespace := flag.String("event-namespace", "gatekeeper-system", "Namespace to listen to events in")
	flag.Parse()

	err := run(*kubeconfig, *eventNamespace)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func run(kubeconfig string, eventNamespace string) error {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return fmt.Errorf("could not create kubeconfig: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("could not create client set: %v", err)
	}

	ctx := context.Background()
	w, err := clientset.EventsV1beta1().Events(eventNamespace).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("could not watch events: %v", err)
	}

	go watchEvents(w)

	http.Handle("/metrics", promhttp.Handler())
	err = http.ListenAndServe(":8888", nil)
	return err
}

func watchEvents(w watch.Interface) {
	violations := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "violation",
		Help: "Violations events from opa-gatekeeper",
	}, []string{"kind", "namespace", "name", "message", "constraint_kind", "constraint_name"})

	for event := range w.ResultChan() {
		eventType := strings.ToLower(string(event.Type))
		if eventType != "added" && eventType != "modified" {
			continue
		}

		e := event.Object.(*eventsv1beta1.Event)
		if e.Reason != "DryrunViolation" && e.Reason != "FailedAdmission" {
			continue
		}

		note := parseNote(e.Note)
		violations.With(map[string]string{
			"kind":            e.Regarding.Kind,
			"namespace":       e.Regarding.Namespace,
			"name":            e.Regarding.Name,
			"message":         note["message"],
			"constraint_kind": e.ObjectMeta.Annotations["constraint_kind"],
			"constraint_name": e.ObjectMeta.Annotations["constraint_name"],
		}).Set(1)
	}
}

func parseNote(note string) map[string]string {
	trimNote := strings.Trim(note, "(combined from similar events): ")
	result := map[string]string{}
	items := strings.SplitN(trimNote, ",", 4)
	for _, item := range items[1:] {
		kv := strings.SplitN(item, ":", 2)
		k := normalizeKey(kv[0])
		result[k] = strings.TrimLeft(kv[1], " ")
	}
	return result
}

func normalizeKey(key string) string {
	k := strings.ToLower(key)
	k = strings.TrimLeft(k, " ")
	k = strings.ReplaceAll(k, " ", "-")
	return k
}
