package modules

import (
	"fmt"
	"strings"
)

const (
	detectorCommandAckDNA     = "6c4401193ea85c"
	detectorCommandAckVersion = "detect-20260404"
)

func buildDetectorCommandAck(deviceID int, command string) string {
	return fmt.Sprintf(
		"device=%d, dna=%s, ver=%s,%s",
		deviceID,
		detectorCommandAckDNA,
		detectorCommandAckVersion,
		strings.TrimSpace(command),
	)
}
