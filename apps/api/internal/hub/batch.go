package hub

import (
	"context"
	"time"
)

func Batches(ctx context.Context, sub *Subscription, interval time.Duration) <-chan []Event {
	out := make(chan []Event)
	go func() {
		defer close(out)
		send := func(batch []Event) bool {
			if len(batch) == 0 {
				return true
			}
			select {
			case out <- batch:
				return true
			case <-ctx.Done():
				return false
			}
		}

		if !send(sub.Replay) {
			return
		}
		for {
			var first Event
			var open bool
			select {
			case first, open = <-sub.C:
			case <-ctx.Done():
				return
			}
			if !open {
				return
			}

			batch := []Event{first}
			timer := time.NewTimer(interval)
		collect:
			for {
				select {
				case ev, open := <-sub.C:
					if !open {
						timer.Stop()
						send(batch)
						return
					}
					batch = append(batch, ev)
				case <-timer.C:
					break collect
				case <-ctx.Done():
					timer.Stop()
					return
				}
			}
			if !send(batch) {
				return
			}
		}
	}()
	return out
}
