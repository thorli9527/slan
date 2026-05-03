package impl

import (
	"sort"
	"strings"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
)

type qualityAccumulator struct {
	count      int
	rttSum     uint64
	rttCount   int
	lossSum    uint64
	lossCount  int
	scoreSum   uint64
	scoreCount int
}

func (a *qualityAccumulator) add(record repo.NodePathHealth) {
	a.count++
	if record.ObservedRttMs != nil {
		a.rttSum += uint64(*record.ObservedRttMs)
		a.rttCount++
	}
	if record.PacketLossPpm != nil {
		a.lossSum += uint64(*record.PacketLossPpm)
		a.lossCount++
	}
	if record.PathScore != nil {
		a.scoreSum += uint64(*record.PathScore)
		a.scoreCount++
	}
}

func (a qualityAccumulator) dto() dto.OpsNetworkQualityCounter {
	out := dto.OpsNetworkQualityCounter{SampleCount: a.count}
	if a.rttCount > 0 {
		out.AvgRttMs = uint32(a.rttSum / uint64(a.rttCount))
	}
	if a.lossCount > 0 {
		out.AvgPacketLossPpm = uint32(a.lossSum / uint64(a.lossCount))
	}
	if a.scoreCount > 0 {
		out.AvgPathScore = uint32(a.scoreSum / uint64(a.scoreCount))
	}
	return out
}

func buildOpsNetworkQualitySummary(records []repo.NodePathHealth, networkNameByID map[string]string) dto.OpsNetworkQualitySummary {
	type networkAgg struct {
		cross            qualityAccumulator
		nonCross         qualityAccumulator
		pathCounts       map[string]int
		activePathCounts map[string]int
		pathDowngrades   uint64
		pathUpgrades     uint64
		pairs            map[string]*qualityAccumulator
		pairMeta         map[string]dto.OpsNetworkQualityCountryPair
	}
	grouped := make(map[string]*networkAgg)
	for _, record := range records {
		agg := grouped[record.NetworkID]
		if agg == nil {
			agg = &networkAgg{
				pathCounts:       make(map[string]int),
				activePathCounts: make(map[string]int),
				pairs:            make(map[string]*qualityAccumulator),
				pairMeta:         make(map[string]dto.OpsNetworkQualityCountryPair),
			}
			grouped[record.NetworkID] = agg
		}
		pathType := strings.TrimSpace(record.PathType)
		if pathType == "" {
			pathType = "unknown"
		}
		agg.pathCounts[pathType]++
		activePath := strings.TrimSpace(record.ActivePath)
		if activePath == "" {
			activePath = pathType
		}
		agg.activePathCounts[activePath]++
		agg.pathDowngrades += record.PathDowngrades
		agg.pathUpgrades += record.PathUpgrades
		isCross := record.CrossCountry != nil && *record.CrossCountry
		if isCross {
			agg.cross.add(record)
		} else {
			agg.nonCross.add(record)
		}
		pairKey := record.SourceCountryCode + "|" + record.RelayCountryCode + "|" + record.PeerCountryCode
		pair := agg.pairs[pairKey]
		if pair == nil {
			pair = &qualityAccumulator{}
			agg.pairs[pairKey] = pair
			agg.pairMeta[pairKey] = dto.OpsNetworkQualityCountryPair{
				SourceCountryCode: record.SourceCountryCode,
				RelayCountryCode:  record.RelayCountryCode,
				PeerCountryCode:   record.PeerCountryCode,
				CrossCountry:      isCross,
			}
		}
		pair.add(record)
	}
	out := make([]dto.OpsNetworkQualityNetworkSummary, 0, len(grouped))
	for networkID, agg := range grouped {
		out = append(out, dto.OpsNetworkQualityNetworkSummary{
			NetworkID:       networkID,
			NetworkName:     networkNameByID[networkID],
			HasCrossCountry: agg.cross.count > 0,
			CrossCountry:    agg.cross.dto(),
			NonCrossCountry: agg.nonCross.dto(),
			PathTypes:       sortedOpsNetworkQualityPathTypes(agg.pathCounts),
			ActivePaths:     sortedOpsNetworkQualityPathTypes(agg.activePathCounts),
			PathDowngrades:  agg.pathDowngrades,
			PathUpgrades:    agg.pathUpgrades,
			CountryPairs:    sortedOpsNetworkQualityCountryPairs(agg.pairs, agg.pairMeta),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].NetworkID < out[j].NetworkID
	})
	return dto.OpsNetworkQualitySummary{Networks: out}
}

func sortedOpsNetworkQualityPathTypes(counts map[string]int) []dto.OpsNetworkQualityPathType {
	out := make([]dto.OpsNetworkQualityPathType, 0, len(counts))
	for pathType, count := range counts {
		out = append(out, dto.OpsNetworkQualityPathType{PathType: pathType, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].PathType < out[j].PathType
	})
	return out
}

func sortedOpsNetworkQualityCountryPairs(
	pairs map[string]*qualityAccumulator,
	pairMeta map[string]dto.OpsNetworkQualityCountryPair,
) []dto.OpsNetworkQualityCountryPair {
	out := make([]dto.OpsNetworkQualityCountryPair, 0, len(pairs))
	for key, acc := range pairs {
		item := pairMeta[key]
		item.Counter = acc.dto()
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Counter.SampleCount != out[j].Counter.SampleCount {
			return out[i].Counter.SampleCount > out[j].Counter.SampleCount
		}
		if out[i].SourceCountryCode != out[j].SourceCountryCode {
			return out[i].SourceCountryCode < out[j].SourceCountryCode
		}
		if out[i].RelayCountryCode != out[j].RelayCountryCode {
			return out[i].RelayCountryCode < out[j].RelayCountryCode
		}
		return out[i].PeerCountryCode < out[j].PeerCountryCode
	})
	return out
}
