package detect

import (
	"os/exec"
	"strconv"
	"strings"
)

// AggregateCPU returns the sum of %CPU for every direct child of the given pid.
// Mirrors the bash one-liner `ps -o pcpu= --ppid PID | awk '{s+=$1} END {print s}'`.
// Returns 0 if pid is invalid or ps fails.
func AggregateCPU(pid int) float64 {
	if pid <= 0 {
		return 0
	}
	out, err := exec.Command("ps", "-o", "pcpu=", "--ppid", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0
	}
	var sum float64
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		v, err := strconv.ParseFloat(line, 64)
		if err != nil {
			continue
		}
		sum += v
	}
	return sum
}
