package advanced

import (
	"fmt"
	"log"
	"time"

	"github.com/prometheus/common/model"

	"github.com/kekexiaoai/inspection/pkg/prom"
)

func RunAdvancedExamples(client *prom.Client) {

	fmt.Println("Advanced Custom Handler Example:")
	customHandlerExample(client)
}

func customHandlerExample(client *prom.Client) {
	now := time.Now()

	customHandler := func(t any) error {
		switch v := t.(type) {
		case *model.Sample:
			labels := make(map[string]string)
			for k, val := range v.Metric {
				labels[string(k)] = string(val)
			}
			fmt.Printf("Sample - Labels: %v, Value: %.2f\n", labels, float64(v.Value))
		case *model.SampleStream:
			fmt.Printf("Stream with %d values\n", len(v.Values))
		default:
			return fmt.Errorf("unexpected type %T", t)
		}
		return nil
	}

	err := prom.ExecuteQuery(client, "up", now, customHandler)
	if err != nil {
		log.Printf("Custom query failed: %v", err)
	}
}
