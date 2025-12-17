package main

import (
	"os"
	"testing"
	"time"

	test "github.com/cert-manager/cert-manager/test/acme"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	zone = os.Getenv("TEST_ZONE_NAME")
)

func TestRunsSuite(t *testing.T) {
	// The manifest path should contain a file named config.json that is a
	// snippet of valid configuration that should be included on the
	// ChallengeRequest passed as part of the test cases.

	log.SetLogger(klog.NewKlogr())

	fixture := test.NewFixture(&active24czSolver{},
		test.SetResolvedZone(zone),
		test.SetStrict(true),
		test.SetPropagationLimit(time.Minute*10),
		test.SetPollInterval(time.Second*15),
		test.SetManifestPath("testdata"),
	)

	fixture.RunConformance(t)
}
