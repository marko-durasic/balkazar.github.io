package gigops

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// A five-minute timer creates at least four legitimate starts per 20 minutes.
// The original three-start limit caused repeated start-limit-hit failures.
func TestCloudTimerStartLimitAllowsNormalCadence(t *testing.T) {
	service, err := os.ReadFile("deploy/musicops-cloud-dryrun.service")
	if err != nil {
		t.Fatal(err)
	}
	timer, err := os.ReadFile("deploy/musicops-cloud-dryrun.timer")
	if err != nil {
		t.Fatal(err)
	}
	unit := string(service)
	if !strings.Contains(string(timer), "OnCalendar=*-*-* *:0/5:00") {
		t.Fatal("timer cadence changed; review start-rate budget against actual schedule")
	}
	if !strings.Contains(unit, "StartLimitIntervalSec=20min") {
		t.Fatal("start-limit interval changed without reviewing cadence")
	}
	m := regexp.MustCompile(`(?m)^StartLimitBurst=([0-9]+)$`).FindStringSubmatch(unit)
	if len(m) != 2 {
		t.Fatal("start-limit burst must be explicit")
	}
	burst, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatal(err)
	}
	if burst < 8 {
		t.Fatalf("start limit %d is below the five-minute normal cadence plus bounded retries; would skip timer executions", burst)
	}
	if !strings.Contains(unit, "RestrictAddressFamilies=AF_UNIX") ||
		!strings.Contains(unit, "NoNewPrivileges=true") ||
		!strings.Contains(unit, "MemoryMax=96M") {
		t.Fatal("dry-run service lost core no-network/security/resource constraints")
	}
}
