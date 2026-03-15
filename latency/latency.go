package latency

import (
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"time"
)

// Measure executes a ping command to measure average RTT to a specific IP.
func Measure(ip string) (time.Duration, error) {
	var args []string
	if runtime.GOOS == "darwin" {
		args = []string{"-c", "4", "-W", "2000"}
	} else {
		args = []string{"-c", "4", "-W", "2"}
	}
	args = append(args, ip)

	cmd := exec.Command("ping", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("ping command failed for IP %s: %v, output: %s", ip, err, string(output))
	}

	re := regexp.MustCompile(`(?:round-trip|rtt) min/avg/max(?:/mdev|/stddev)? = ([0-9.]+)/([0-9.]+)/`)
	matches := re.FindStringSubmatch(string(output))
	if len(matches) < 3 {
		return 0, fmt.Errorf("could not parse ping average RTT for IP %s: %s", ip, string(output))
	}

	avgMs, err := strconv.ParseFloat(matches[2], 64)
	if err != nil {
		return 0, fmt.Errorf("could not convert ping average RTT to float for IP %s: %v", ip, err)
	}

	return time.Duration(avgMs * float64(time.Millisecond)), nil
}
