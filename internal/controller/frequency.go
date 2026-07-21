package controller

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const defaultFrequency = 60 * time.Second

// frequencyOrDefault returns the configured frequency, or defaultFrequency
// if the field is unset (e.g. for CRs created before it existed).
func frequencyOrDefault(f *metav1.Duration) time.Duration {
	if f == nil {
		return defaultFrequency
	}

	return f.Duration
}
