package main

func pickOneAction(acts []action) (bool, *action) {
	totalWeight := 0
	for _, v := range acts {
		if v.cnt == 0 {
			continue
		}
		if !UseWeightPolicy {
			totalWeight += 1
		} else {
			totalWeight += v.weight
		}
	}
	if totalWeight == 0 {
		return false, nil
	}
	target := genRandInt(totalWeight) + 1 // the value in the [1,totalWeight]
	now := 0
	for i := range acts {
		v := acts[i]
		if v.cnt == 0 {
			continue
		}
		if !UseWeightPolicy {
			now += 1
		} else {
			now += v.weight
		}
		if now >= target {
			return true, &acts[i]
		}
	}
	return false, nil
}
