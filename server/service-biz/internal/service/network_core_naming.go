package service

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/slan/service-biz/internal/model"
)

var managedNetworkSequencePattern = regexp.MustCompile(`(?i)^network-(\d+)$`)

func nextManagedNetworkName(items []model.Network) string {
	maxSequence := 0
	for _, item := range items {
		matches := managedNetworkSequencePattern.FindStringSubmatch(strings.TrimSpace(item.Name))
		if len(matches) != 2 {
			continue
		}
		sequence, err := strconv.Atoi(matches[1])
		if err == nil && sequence > maxSequence {
			maxSequence = sequence
		}
	}
	return fmt.Sprintf("network-%02d", maxSequence+1)
}
