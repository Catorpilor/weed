package schedule

import (
    "math/rand"
    "time"
)

type Scheduler struct {
    base   time.Duration
    jitter float64
    ch     chan time.Time
}

// New returns a scheduler that ticks at base ± jitter%.
func New(base time.Duration, jitterPct float64) *Scheduler {
    s := &Scheduler{base: base, jitter: jitterPct, ch: make(chan time.Time)}
    go s.loop()
    return s
}

func (s *Scheduler) Next() <-chan time.Time { return s.ch }

func (s *Scheduler) loop() {
    for {
        // jitter in [-j, +j]
        j := (rand.Float64()*2 - 1) * s.jitter
        d := time.Duration(float64(s.base) * (1 + j))
        if d < time.Second {
            d = time.Second
        }
        t := time.NewTimer(d)
        <-t.C
        s.ch <- time.Now()
    }
}

