package utils

func GetSlotTimestamp(slot, genesisTimestamp, slotDuration uint64) uint64 {
	return genesisTimestamp + (slot)*slotDuration
}

func GetSlotForBlock(blockTimestamp, genesisTimestamp, slotDuration uint64) uint64 {
	return (blockTimestamp - genesisTimestamp) / slotDuration
}

func GetSlotNumber(currentTime, genesisTime, slotDuration uint64) uint64 {
	return (currentTime - genesisTime) / slotDuration
}

func GetEpochNumber(slotNumber, slotsPerEpoch uint64) uint64 {
	return slotNumber / slotsPerEpoch
}

func GetFirstSlotOfEpoch(epochNumber, slotsPerEpoch uint64) uint64 {
	return epochNumber * slotsPerEpoch
}

func GetLastSlotOfEpoch(epochNumber, slotsPerEpoch uint64) uint64 {
	return (epochNumber+1)*slotsPerEpoch - 1
}
