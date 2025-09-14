package timers

import (
	"sync"
	"time"
)

type Timer struct {
	ID       string
	Rounds   int
	Interval time.Duration
	Task     func(map[string]any)
	Args     map[string]any
	Repeat   bool
	stop     chan struct{}
}

type slot struct {
	timers map[string]*Timer
}

type TimingWheel struct {
	tickDuration time.Duration
	slots        []*slot
	currentPos   int
	slotsCount   int
	mutex        sync.Mutex
	ticker       *time.Ticker
}

func NewTimingWheel(tickDuration time.Duration, slotsCount int) *TimingWheel {
	tw := &TimingWheel{
		tickDuration: tickDuration,
		slotsCount:   slotsCount,
		slots:        make([]*slot, slotsCount),
		currentPos:   0,
	}

	for i := range tw.slots {
		tw.slots[i] = &slot{timers: make(map[string]*Timer)}
	}

	tw.ticker = time.NewTicker(tickDuration)
	go tw.start()
	return tw
}

func (tw *TimingWheel) start() {
	for range tw.ticker.C {
		tw.mutex.Lock()
		currentSlot := tw.slots[tw.currentPos]

		for id, timer := range currentSlot.timers {
			if timer.Rounds > 0 {
				timer.Rounds--
				continue
			}
			go timer.Task(timer.Args)

			if timer.Repeat {
				nextPos := (tw.currentPos + int(timer.Interval/tw.tickDuration)) % tw.slotsCount
				tw.slots[nextPos].timers[id] = timer
			}
			delete(currentSlot.timers, id)
		}

		tw.currentPos = (tw.currentPos + 1) % tw.slotsCount
		tw.mutex.Unlock()
	}
}

func (tw *TimingWheel) AddTimer(id string, interval time.Duration, repeat bool, args map[string]any, task func(map[string]any)) {
	tw.mutex.Lock()
	defer tw.mutex.Unlock()

	t := &Timer{
		ID:       id,
		Interval: interval,
		Task:     task,
		Args:     args,
		Repeat:   repeat,
		stop:     make(chan struct{}),
	}

	pos := (tw.currentPos + int(interval/tw.tickDuration)) % tw.slotsCount
	rounds := int(interval/tw.tickDuration) / tw.slotsCount

	t.Rounds = rounds
	tw.slots[pos].timers[id] = t

}

func (tw *TimingWheel) RemoveTimer(id string) {
	tw.mutex.Lock()
	defer tw.mutex.Unlock()

	for _, s := range tw.slots {
		delete(s.timers, id)
	}
}
