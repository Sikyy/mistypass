package retrybackoff

import "time"

func Normalize(base, max time.Duration) (time.Duration, time.Duration) {
	if base < 0 {
		base = 0
	}
	if base == 0 {
		return 0, 0
	}
	if max <= 0 || max < base {
		max = base
	}
	return base, max
}

func Exponential(attempt int, base, max time.Duration) time.Duration {
	base, max = Normalize(base, max)
	if attempt <= 0 || base <= 0 {
		return 0
	}

	delay := base
	for i := 1; i < attempt; i++ {
		if delay >= max {
			return max
		}
		if delay > max/2 {
			return max
		}
		delay *= 2
	}
	if delay > max {
		return max
	}
	return delay
}
