package util

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/minerdao/lotus-car/db"
)

// ParseDealResponse parses the deal response string and returns a Deal struct
func ParseDealResponse(response string) (*db.Deal, error) {
	lines := strings.Split(response, "\n")
	deal := &db.Deal{
		Status: "proposed", // Initial status when deal is created
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "deal uuid":
			deal.UUID = value
		case "storage provider":
			deal.StorageProvider = value
		case "client wallet":
			deal.ClientWallet = value
		case "payload cid":
			deal.PayloadCid = value
		case "commp":
			deal.CommP = value
		case "start epoch":
			epoch, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse start epoch: %v", err)
			}
			deal.StartEpoch = epoch
		case "end epoch":
			epoch, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse end epoch: %v", err)
			}
			deal.EndEpoch = epoch
		case "provider collateral":
			// Parse "X.XXX mFIL" format
			collateralStr := strings.Split(value, " ")[0]
			collateral, err := strconv.ParseFloat(collateralStr, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse provider collateral: %v", err)
			}
			deal.ProviderCollateral = collateral
		}
	}

	// Validate required fields
	if deal.UUID == "" {
		return nil, fmt.Errorf("deal UUID not found in response")
	}

	return deal, nil
}
